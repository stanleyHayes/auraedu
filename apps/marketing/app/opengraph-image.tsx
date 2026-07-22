import { ImageResponse } from "next/og";

export const alt = "AuraEDU — the education operating system";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OpenGraphImage() {
  return new ImageResponse(
    <div
      style={{
        width: "100%",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        justifyContent: "space-between",
        background: "#061631",
        color: "white",
        padding: "68px 76px",
        fontFamily: "Arial, sans-serif",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 20 }}>
        <div
          style={{
            width: 58,
            height: 58,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            border: "4px solid #63D5DA",
            borderRadius: 18,
            color: "#B7F500",
            fontSize: 30,
            fontWeight: 900,
          }}
        >
          A
        </div>
        <div style={{ display: "flex", fontSize: 38, fontWeight: 800, letterSpacing: -1 }}>
          AuraEDU
        </div>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 28 }}>
        <div
          style={{ display: "flex", maxWidth: 980, fontSize: 76, fontWeight: 800, lineHeight: 1 }}
        >
          Run your school clearly.
        </div>
        <div
          style={{
            display: "flex",
            maxWidth: 900,
            fontSize: 31,
            lineHeight: 1.35,
            color: "#CBD5E1",
          }}
        >
          Operations, learning, families, growth and accountable AI on one trusted foundation.
        </div>
      </div>
      <div
        style={{ display: "flex", gap: 12, alignItems: "center", fontSize: 22, color: "#63D5DA" }}
      >
        <span style={{ color: "#B7F500" }}>●</span> The education operating system
      </div>
    </div>,
    size,
  );
}
