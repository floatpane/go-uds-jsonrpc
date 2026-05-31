import type { Metadata } from "next";
import "./globals.css";
import "highlight.js/styles/github-dark.css";

export const metadata: Metadata = {
	title: "go-uds-jsonrpc",
	description:
		"Tiny newline-delimited JSON-RPC over Unix domain sockets for Go. Daemon ↔ client with request/response + server-pushed events, PID files, and XDG-aware socket paths.",
};

export default function RootLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<html lang="en">
			<body>
				<header className="site-header">
					<a href="/" className="brand">
						go-uds-jsonrpc
					</a>
					<nav>
						<a href="https://github.com/floatpane/go-uds-jsonrpc">GitHub</a>
					</nav>
				</header>
				<main>{children}</main>
			</body>
		</html>
	);
}

export const viewport = { width: "device-width", initialScale: 1 };
