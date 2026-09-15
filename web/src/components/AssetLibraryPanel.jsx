import { useCallback, useEffect, useState } from "react";
import { api } from "../api/client";
import { useLanguage } from "../i18n/LanguageContext";

export default function AssetLibraryPanel({ onClose, onAdd }) {
  const { t, locale } = useLanguage();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [adding, setAdding] = useState("");
  const [groupId, setGroupId] = useState("");
  const refresh = useCallback(async () => {
    setLoading(true); setError("");
    try { setData(await api.assetLibrary(1, groupId)); }
    catch (err) { setError(err.message); }
    finally { setLoading(false); }
  }, [groupId]);
  useEffect(() => { refresh(); }, [refresh]);
  const statusLabel = (status) => {
    const key = `assetStatus${String(status || "unknown").replace(/(^|_)(\w)/g, (_, __, c) => c.toUpperCase())}`;
    return t(key) === key ? status || t("unknown") : t(key);
  };
  return (
    <aside className="task-panel asset-library-panel" aria-label={t("trustedAssetLibrary")}>
      <div className="task-panel-heading">
        <div><h2>{t("trustedAssetLibrary")}</h2><p>{data ? `${data.projectName} · ${t("remoteAssetCount", { count: data.summary.remoteTotal })}` : t("assetLibraryLead")}</p></div>
        <button aria-label={t("closeTaskPanel")} onClick={onClose}>×</button>
      </div>
      <div className="asset-library-toolbar">
        <span>{data?.credentialScope || "—"}</span>
        <button onClick={refresh} disabled={loading}>{loading ? t("syncing") : t("syncNow")}</button>
      </div>
      {data && <div className="asset-group-picker">
        <label>{t("assetGroup")}<select value={groupId} onChange={(event) => setGroupId(event.target.value)}><option value="">{t("allAssetGroups")}</option>{data.groups.map((group) => <option value={group.id} key={group.id}>{group.name || group.title || group.id}</option>)}</select></label>
        <small>{t("assetGroupCount", { count: data.groupTotal })}</small>
        {groupId && <p>{data.groups.find((group) => group.id === groupId)?.description || data.groups.find((group) => group.id === groupId)?.id}</p>}
      </div>}
      {data && <div className="asset-library-summary">
        <span><b>{data.summary.active}</b>{t("assetActive")}</span>
        <span><b>{data.summary.processing}</b>{t("assetProcessing")}</span>
        <span><b>{data.summary.failed}</b>{t("assetFailed")}</span>
      </div>}
      {error && <p className="task-error">{error}</p>}
      {!loading && data?.items.length === 0 && <p className="panel-empty">{t("assetLibraryEmpty")}</p>}
      <div className="asset-library-list">
        {data?.items.map((item) => <article className="asset-library-card" key={item.id}>
          <div><strong>{item.name || item.id}</strong><span className={`asset-status ${String(item.status).toLowerCase()}`}>{statusLabel(item.status)}</span></div>
          <code>asset://{item.id}</code>
          <small>{item.assetType || "—"} · {data.groups.find((group) => group.id === item.groupId)?.name || item.groupId || t("ungrouped")}</small>
          <small>{item.updateTime ? new Date(item.updateTime).toLocaleString(locale, { hour12: false }) : "—"}</small>
          {["active"].includes(String(item.status).toLowerCase()) && ["image", "video", "audio"].some((type) => String(item.assetType).toLowerCase().includes(type)) && <button className="asset-add-button" disabled={adding === item.id} onClick={async () => { setAdding(item.id); setError(""); try { await onAdd(item); } catch (err) { setError(err.message); } finally { setAdding(""); } }}>{adding === item.id ? t("addingToCanvas") : t("addToCanvas")}</button>}
        </article>)}
      </div>
      <p className="task-footnote">{t("assetLibrarySyncHelp")}</p>
    </aside>
  );
}
