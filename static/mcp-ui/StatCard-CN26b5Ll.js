import { j as n } from "./client-BV8LgGti.js";
(function() {
  var e = typeof globalThis < "u" ? globalThis : typeof window < "u" ? window : typeof global < "u" ? global : this;
  e && typeof e.process > "u" && (e.process = { env: { NODE_ENV: "production" } });
})();
const i = {
  borderRadius: "12px",
  padding: "16px",
  background: "rgba(255, 255, 255, 0.05)",
  border: "1px solid rgba(255, 255, 255, 0.08)",
  boxShadow: "0 4px 16px rgba(0, 0, 0, 0.08)",
  display: "flex",
  flexDirection: "column",
  gap: "8px"
}, l = {
  fontSize: "14px",
  color: "#7a7a7a",
  textTransform: "uppercase",
  letterSpacing: "0.08em"
}, r = {
  fontSize: "24px",
  fontWeight: 600,
  color: "#f5f5f5"
}, s = {
  fontSize: "13px",
  fontWeight: 500,
  display: "inline-flex",
  alignItems: "center",
  gap: "4px"
}, a = (e) => e === "up" ? "▲" : "▼", p = (e) => e === "up" ? "#4CAF50" : "#EF5350";
function c({ value: e, label: t, trend: o }) {
  return /* @__PURE__ */ n.jsxs("div", { style: i, children: [
    /* @__PURE__ */ n.jsx("span", { style: l, children: t }),
    /* @__PURE__ */ n.jsx("span", { style: r, children: e }),
    o && /* @__PURE__ */ n.jsxs("span", { style: { ...s, color: p(o.direction) }, children: [
      a(o.direction),
      o.value
    ] })
  ] });
}
export {
  c as S
};
