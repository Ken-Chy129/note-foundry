import type { Metadata } from "next";
import { WorkspaceApp } from "@/components/workspace/WorkspaceApp";

export const metadata: Metadata = { title: "Owner workspace", robots: { index: false, follow: false } };

export default function AppPage() {
  return <WorkspaceApp />;
}
