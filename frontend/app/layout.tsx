import type { Metadata } from "next";
import "./globals.css";
import { PreferencesProvider } from "../components/PreferencesContext";

export const metadata: Metadata = {
  title: "Ariba Shield WAF — Enterprise Security Control Plane",
  description:
    "Enterprise Web Application Firewall management console for policies, websites, upstreams, RBAC, and session security."
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" data-theme="dark" suppressHydrationWarning>
      <body>
        <PreferencesProvider>{children}</PreferencesProvider>
      </body>
    </html>
  );
}
