import { Alert, Button } from "@xaroth.nl/design/react";

export function ErrorPage({ message }: { message: string }) {
  return (
    <>
      <h1>Login failed</h1>
      <Alert tone="danger">{message}</Alert>
      <Button href="/">Start again</Button>
    </>
  );
}
