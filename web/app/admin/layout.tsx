import type { ReactNode } from "react";

const links = [
  ["/admin", "Overview"],
  ["/admin/system", "System"],
  ["/admin/support", "Support"],
  ["/admin/datahub", "Data Hub"],
  ["/admin/organizations", "Organizations"],
  ["/admin/reviews", "Reviews"],
] as const;

export default function AdminLayout({ children }: { children: ReactNode }) {
  return <>
    <nav aria-label="Admin sections" style={{ maxWidth: 1180, margin: "16px auto 0", padding: "0 24px", display: "flex", gap: 8, flexWrap: "wrap" }}>
      {links.map(([href, label]) => <a key={href} href={href} style={{ padding: "7px 10px", border: "1px solid #ddd", borderRadius: 8, color: "#111", textDecoration: "none", background: "#fff" }}>{label}</a>)}
    </nav>
    {children}
  </>;
}
