import { ImageResponse } from "next/og"
import { readFile } from "node:fs/promises"
import { join } from "node:path"

export const revalidate = false
export const size = { width: 1200, height: 630 }
export const contentType = "image/png"
export const alt = "runtz — DevSecOps Platform"

// Official lockup (Runtz Logo design project): JetBrains Mono 700,
// tracking -0.05em, cursor 0.44em x 0.82em, gap 0.2em, no radius.
const wordmarkSize = 148

export default async function Image() {
  const jetbrainsMono = await readFile(join(process.cwd(), "src/lib/fonts/JetBrainsMono-Bold.ttf"))

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          padding: "0 96px",
          backgroundColor: "#050912",
          backgroundImage:
            "radial-gradient(ellipse 80% 60% at 20% 110%, rgba(47, 126, 255, 0.28), transparent)",
          fontFamily: "JetBrains Mono",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: wordmarkSize * 0.2 }}>
          <div
            style={{
              display: "flex",
              fontSize: wordmarkSize,
              fontWeight: 700,
              color: "#ffffff",
              letterSpacing: "-0.05em",
            }}
          >
            runtz
          </div>
          <div
            style={{
              display: "flex",
              width: wordmarkSize * 0.44,
              height: wordmarkSize * 0.82,
              backgroundColor: "#6db5ff",
              marginTop: wordmarkSize * 0.07,
            }}
          />
        </div>
        <div style={{ display: "flex", marginTop: 36, fontSize: 34, color: "#b8cbe4" }}>
          DevSecOps Platform
        </div>
        <div style={{ display: "flex", marginTop: 18, fontSize: 26, color: "#6db5ff" }}>
          SCA · SAST · Host · Container · Kubernetes
        </div>
        <div
          style={{
            position: "absolute",
            bottom: 48,
            left: 96,
            display: "flex",
            fontSize: 24,
            color: "#53657d",
          }}
        >
          runtz.dev
        </div>
      </div>
    ),
    {
      ...size,
      fonts: [{ name: "JetBrains Mono", data: jetbrainsMono, weight: 700, style: "normal" }],
    },
  )
}
