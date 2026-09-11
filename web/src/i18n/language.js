export function preferredLanguage(saved, browserLanguage = "") {
  if (saved === "zh" || saved === "en") return saved;
  return browserLanguage.toLowerCase().startsWith("zh") ? "zh" : "en";
}
