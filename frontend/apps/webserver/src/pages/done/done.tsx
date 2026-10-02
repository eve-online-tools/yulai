import { Button } from "@xaroth.nl/design/react";
import { Portrait } from "@yulai/ui";

export function DonePage({ id, name }: { id: number; name: string }) {
  return (
    <>
      <h1>Character added</h1>
      <div className="row">
        <Portrait id={id} size={48} />
        <span>{name} is logged in. You can close this tab.</span>
      </div>
      <Button href="/">Add another character</Button>
      <p className="muted small">
        EVE SSO remembers your account. To add a character from another account, log out at login.eveonline.com first.
      </p>
    </>
  );
}
