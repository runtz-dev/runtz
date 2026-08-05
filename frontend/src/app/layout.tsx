import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono, JetBrains_Mono } from "next/font/google";
import Script from "next/script";
import { ThemeProvider } from "@/components/runtz/theme-provider";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

// Brand wordmark font (official lockup: JetBrains Mono 700).
const brandMono = JetBrains_Mono({
  variable: "--font-brand",
  subsets: ["latin"],
  weight: ["700"],
});

const platformDescription =
  "Open source DevSecOps platform: SCA, SAST, host, container and Kubernetes scan dashboards, workspaces and CLI API keys.";

export const metadata: Metadata = {
  metadataBase: new URL("https://runtz.dev"),
  applicationName: "runtz",
  title: {
    default: "runtz — DevSecOps Platform",
    template: "%s — runtz",
  },
  description: platformDescription,
  authors: [{ name: "RAW DevOps", url: "https://runtz.dev" }],
  creator: "RAW DevOps",
  publisher: "RAW DevOps",
  category: "technology",
  openGraph: {
    type: "website",
    siteName: "runtz",
    locale: "en_US",
    title: "runtz — DevSecOps Platform",
    description: platformDescription,
  },
  twitter: {
    card: "summary_large_image",
    site: "@runtz",
    creator: "@runtz",
  },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#eef6ff" },
    { media: "(prefers-color-scheme: dark)", color: "#050912" },
  ],
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} ${brandMono.variable} h-full antialiased`}
      suppressHydrationWarning
    >
      <body className="min-h-full flex flex-col">
        <Script id="runtz-theme" strategy="beforeInteractive">
          {`try{var t=localStorage.getItem('runtz_theme');if(t==='white'){document.documentElement.style.colorScheme='light'}else{document.documentElement.classList.add('dark');document.documentElement.style.colorScheme='dark'}}catch(e){document.documentElement.classList.add('dark');document.documentElement.style.colorScheme='dark'}`}
        </Script>
        <ThemeProvider>{children}</ThemeProvider>
      </body>
    </html>
  );
}
