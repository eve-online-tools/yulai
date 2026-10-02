import type { ReactNode } from "react";
import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Service as Characters } from "@bindings/github.com/eve-online-tools/yulai/feature/character";
import type { SetupStatus } from "@bindings/github.com/eve-online-tools/yulai/app";
import { Portrait } from "@yulai/ui";
import { charactersQuery, setupQuery } from "../queries";

// Covers only what blocks using the app. Optional steps live in the top bar checklist.
export function WelcomePage() {
  const { data: setup } = useSuspenseQuery(setupQuery);
  const { data: characters } = useSuspenseQuery(charactersQuery);
  const first = characters[0];

  return (
    <div className="welcome">
      <div className="welcome-panel">
        <h1>Welcome to Yulai</h1>
        <p className="muted">Two steps before you can start.</p>
        <ol className="steps">
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
      </div>
    </div>
  );
}

type StepState = "todo" | "active" | "done";

function Step({ n, title, state, children }: { n: number; title: string; state: StepState; children?: ReactNode }) {
  return (
    <li className={`step ${state}`}>
      <span className="step-n">{state === "done" ? "✓" : n}</span>
      <div className="step-body">
        <div className="step-title">{title}</div>
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
        <button onClick={() => add.mutate()} disabled={add.isPending}>
          Add character
        </button>
        {add.isSuccess && <span className="muted small">Waiting for login in your browser...</span>}
      </div>
      {add.isError && <p className="error small">{String(add.error)}</p>}
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
      <Portrait id={id} size={48} className="avatar" />
      <span>Welcome, {name}.</span>
      <button className="push-right" onClick={() => navigate({ to: "/characters" })}>
        Continue
      </button>
    </div>
  );
}

function Copyable({ text }: { text: string }) {
  return (
    <code className="copyable" title="Click to copy" onClick={() => navigator.clipboard.writeText(text)}>
      {text}
    </code>
  );
}
