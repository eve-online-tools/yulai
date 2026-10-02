import type { ReactNode } from "react";
import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Service as Characters } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import type { SetupStatus } from "@bindings/github.com/eve-online-tools/yulai/app";
import { Alert, Button, Panel } from "@xaroth.nl/design/react";
import { Portrait } from "@yulai/ui";
import { charactersQuery, setupQuery } from "../../queries";
import styles from "./welcome.module.scss";

// Covers only what blocks using the app. Optional steps live in the top bar checklist.
export function WelcomePage() {
  const { data: setup } = useSuspenseQuery(setupQuery);
  const { data: characters } = useSuspenseQuery(charactersQuery);
  const first = characters[0];

  return (
    <div className={styles.welcome}>
      <Panel variant="raised" marks padding="lg" className={styles.panel}>
        <p className={styles.eyebrow}>First run</p>
        <h1>Welcome to Yulai</h1>
        <p className="muted">Two steps before you can start.</p>
        <ol className={styles.steps}>
          <Step n={1} title="Register an SSO application" state={setup.ssoConfigured ? "done" : "active"}>
            {!setup.ssoConfigured && <SSOInstructions setup={setup} />}
          </Step>
          <Step n={2} title="Add your first character" state={first ? "done" : setup.ssoConfigured ? "active" : "todo"}>
            {setup.ssoConfigured && (first ? <Added name={first.name} id={first.id} /> : <AddFirst loginUrl={setup.loginUrl} />)}
          </Step>
        </ol>
        <p className="muted small">
          Login runs in your own browser, so Yulai never sees your password. Tokens are stored encrypted on this machine.
        </p>
      </Panel>
    </div>
  );
}

type StepState = "todo" | "active" | "done";

function Step({ n, title, state, children }: { n: number; title: string; state: StepState; children?: ReactNode }) {
  return (
    <li className={styles.step} data-state={state}>
      <span className={styles.n}>{state === "done" ? "✓" : n}</span>
      <div className={styles.body}>
        <div className={styles.title}>{title}</div>
        {children}
      </div>
    </li>
  );
}

function SSOInstructions({ setup }: { setup: SetupStatus }) {
  return (
    <ol className="small">
      <li>
        Create an application at{" "}
        <a href="https://developers.eveonline.com" target="_blank" rel="noreferrer">
          developers.eveonline.com
        </a>{" "}
        with callback URL <Copyable text={setup.callbackUrl} />
      </li>
      <li>
        Save its client ID as <code>{`{ "clientId": "..." }`}</code> in <Copyable text={setup.ssoPath} />
      </li>
      <li>Restart Yulai.</li>
    </ol>
  );
}

function AddFirst({ loginUrl }: { loginUrl: string }) {
  const add = useMutation({ mutationFn: () => Characters.AddCharacter() });
  return (
    <>
      <p className="muted small">Pick the features you want, then log in with EVE SSO in your browser.</p>
      <div className="row">
        <Button onClick={() => add.mutate()} loading={add.isPending}>
          Add character
        </Button>
        {add.isSuccess && <span className="muted small">Waiting for login in your browser...</span>}
      </div>
      {add.isError && <Alert tone="danger">{String(add.error)}</Alert>}
      {(add.isSuccess || add.isError) && (
        <p className="muted small">
          Browser did not open? Go to <Copyable text={loginUrl} />
        </p>
      )}
    </>
  );
}

function Added({ id, name }: { id: number; name: string }) {
  const navigate = useNavigate();
  return (
    <div className="row">
      <Portrait id={id} size={48} />
      <span>Welcome, {name}.</span>
      <Button className="push-right" onClick={() => navigate({ to: "/characters" })}>
        Continue
      </Button>
    </div>
  );
}

function Copyable({ text }: { text: string }) {
  return (
    <code className={styles.copyable} title="Click to copy" onClick={() => navigator.clipboard.writeText(text)}>
      {text}
    </code>
  );
}
