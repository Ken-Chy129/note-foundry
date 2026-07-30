import type { Metadata } from "next";
import { WorkspaceApp } from "@/components/workspace/WorkspaceApp";

export const metadata: Metadata = { title: "所有者工作区", robots: { index: false, follow: false } };

export default function WorkspacePage() {
  return <WorkspaceApp />;
}
