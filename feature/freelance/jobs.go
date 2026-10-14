package freelance

import (
	"context"
	"net/http"
	"time"

	getjobs "github.com/eve-online-tools/lib-esi-go/esi/getfreelancejobslisting"
	"github.com/eve-online-tools/lib-esi-go/request"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

const listingPublic = "public"

// ListingInput is the input of the public crawl, which has one subject.
type ListingInput struct{}

func (ListingInput) Subject() string { return "freelance:" + listingPublic }

// requestJobs is getjobs.Request with value cursor fields: lib-esi-go fails to
// encode pointer query parameters like those of getjobs.Input.
var requestJobs = request.Create[jobsInput, *getjobs.Output](
	http.MethodGet,
	"/freelance-jobs",
)

type jobsInput struct {
	After  string `query:"after"`
	Before string `query:"before"`
}

// Crawl is what one run of FetchJobs did.
type Crawl struct {
	Pages   int `json:"pages"`
	Changed int `json:"changed"`
}

var FetchJobs = task.New(
	(*Freelance).jobs,
	task.WithStartup(),
	task.WithInterval(every),
	task.WithTimeout(crawlTimeout),
	task.WithProgress(ProgressKey),
)

// jobs crawls the cursor-paginated public listing. The first run takes the newest
// page, then backfills older pages through before. Later runs follow after from
// the saved token and get only what changed since. Each page is stored and
// announced on its own, so the frontend fills in while the crawl runs.
func (f *Freelance) jobs(ctx context.Context, in ListingInput) (*Crawl, error) {
	cur, err := f.q.GetCursor(ctx, listingPublic)
	_, err = db.Found(err)
	if err != nil {
		return nil, err
	}
	cur.Listing = listingPublic
	crawl := &Crawl{}

	if cur.AfterToken == nil {
		data, err := f.jobsPage(ctx, &jobsInput{})
		if err != nil {
			return nil, err
		}
		next := FreelanceCursor{Listing: listingPublic}
		if data.Cursor != nil {
			next.AfterToken = data.Cursor.After
			if len(data.FreelanceJobs) > 0 {
				next.BeforeToken = data.Cursor.Before
			}
		}
		if err := f.storePage(ctx, crawl, data.FreelanceJobs, next); err != nil {
			return nil, err
		}
		cur = next
	}

	for cur.BeforeToken != nil {
		task.Report(ctx, task.Progress{Phase: "backfill", Done: int64(crawl.Pages)})
		data, err := f.jobsPage(ctx, &jobsInput{Before: *cur.BeforeToken})
		if err != nil {
			return nil, err
		}
		next := cur
		next.BeforeToken = nil
		if len(data.FreelanceJobs) > 0 && data.Cursor != nil && !sameToken(data.Cursor.Before, cur.BeforeToken) {
			next.BeforeToken = data.Cursor.Before
		}
		if err := f.storePage(ctx, crawl, data.FreelanceJobs, next); err != nil {
			return nil, err
		}
		cur = next
	}

	for {
		task.Report(ctx, task.Progress{Phase: "update", Done: int64(crawl.Pages)})
		data, err := f.jobsPage(ctx, &jobsInput{After: *cur.AfterToken})
		if err != nil {
			return nil, err
		}
		// Only an empty page ends the crawl; a short one does not.
		if len(data.FreelanceJobs) == 0 {
			break
		}
		next := cur
		if data.Cursor != nil && data.Cursor.After != nil {
			next.AfterToken = data.Cursor.After
		}
		if err := f.storePage(ctx, crawl, data.FreelanceJobs, next); err != nil {
			return nil, err
		}
		if sameToken(next.AfterToken, cur.AfterToken) {
			break
		}
		cur = next
	}
	return crawl, nil
}

func (f *Freelance) jobsPage(ctx context.Context, in *jobsInput) (*getjobs.Output, error) {
	resp, err := requestJobs(
		ctx, f.esi,
		in,
	)
	if err != nil {
		return nil, err
	}
	if err := esi.ResponseError(resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// storePage writes a page and the cursor after it in one transaction, so an
// interrupted crawl resumes at the first page it did not store. A record already
// stored with a later last_modified is newer and kept.
func (f *Freelance) storePage(ctx context.Context, crawl *Crawl, jobs []getjobs.FreelanceJobsDetailFreelancejob, cur FreelanceCursor) error {
	changed := 0
	now := time.Now().UTC()
	err := f.inTx(ctx, func(q *Queries) error {
		for _, j := range jobs {
			prev, err := q.GetJob(ctx, j.Id)
			existed, err := db.Found(err)
			if err != nil {
				return err
			}
			if existed && prev.LastModified.After(j.LastModified) {
				continue
			}
			p := UpsertJobParams{
				ID:              j.Id,
				Name:            j.Name,
				State:           j.State,
				ProgressCurrent: j.Progress.Current,
				ProgressDesired: j.Progress.Desired,
				LastModified:    j.LastModified.UTC(),
				FetchedAt:       now,
			}
			if j.Reward != nil {
				p.RewardInitial = &j.Reward.Initial
				p.RewardRemaining = &j.Reward.Remaining
			}
			row, err := q.UpsertJob(ctx, p)
			if err != nil {
				return err
			}
			if !existed || db.Changed(prev, row) {
				changed++
			}
		}
		return q.UpsertCursor(ctx, UpsertCursorParams(cur))
	})
	if err != nil {
		return err
	}
	crawl.Pages++
	crawl.Changed += changed
	if changed > 0 {
		f.events.Changed(Topic)
	}
	return nil
}

func sameToken(a, b *string) bool {
	return a != nil && b != nil && *a == *b
}
