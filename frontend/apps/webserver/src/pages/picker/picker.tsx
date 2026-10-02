import { Button, Checkbox, CheckboxGroup } from "@xaroth.nl/design/react";
import type { Feature } from "@yulai/ui";
import styles from "./picker.module.scss";

// A plain form post, so /start can redirect the browser straight to the SSO.
export function PickerPage({ csrf, features }: { csrf: string; features: Feature[] }) {
  return (
    <>
      <h1>Add character</h1>
      <p className="muted">Pick what this character should share. Logging in again later replaces this choice.</p>
      <form method="post" action="/start" className={styles.form}>
        <input type="hidden" name="csrf" value={csrf} />
        {features.length > 0 ? (
          <CheckboxGroup id="features" legend="Features">
            {features.map((f) => (
              <Checkbox key={f.name} name="feature" value={f.name} defaultChecked>
                <span>{f.name}</span>
                <span className="muted small"> {f.scopes.join(", ")}</span>
              </Checkbox>
            ))}
          </CheckboxGroup>
        ) : (
          <p className="muted">No features yet. Logging in only identifies the character.</p>
        )}
        <Button type="submit">Log in with EVE Online</Button>
      </form>
    </>
  );
}
