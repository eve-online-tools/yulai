import { Button } from "@xaroth.nl/design/react";
import { Portrait } from "@yulai/ui";

// "Add another" reposts the previous choice, so the browser goes straight to the SSO.
export function DonePage({ id, name, csrf, features }: { id: number; name: string; csrf: string; features: string[] }) {
  return (
    <>
      <h1>Character added</h1>
      <div className="row">
        <Portrait id={id} size={48} />
        <span>{name} is logged in. You can close this tab.</span>
      </div>
      <form method="post" action="/start">
        <input type="hidden" name="csrf" value={csrf} />
        {features.map((f) => (
          <input key={f} type="hidden" name="feature" value={f} />
        ))}
        <Button type="submit">Add another character</Button>
      </form>
      <a href="/" className="small">
        Back
      </a>
    </>
  );
}
