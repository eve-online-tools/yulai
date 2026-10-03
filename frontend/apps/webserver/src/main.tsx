import React from "react";
import ReactDOM from "react-dom/client";
import "@xaroth.nl/design/themes/eve-online.css";
import "@yulai/ui/styles.css";
import { Page } from "./components/page";
import { DonePage } from "./pages/done";
import { ErrorPage } from "./pages/error";
import { PickerPage } from "./pages/picker";
import { readPageData } from "./page-data";

const data = readPageData();

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <Page>
      {data.page === "picker" && <PickerPage csrf={data.csrf} features={data.features} />}
      {data.page === "done" && <DonePage id={data.id} name={data.name} csrf={data.csrf} features={data.features} />}
      {data.page === "error" && <ErrorPage message={data.message} />}
    </Page>
  </React.StrictMode>,
);
