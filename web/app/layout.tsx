import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Поиск",
  description: "Независимый Search + Answer Engine",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru">
      <body>{children}</body>
    </html>
  );
}
