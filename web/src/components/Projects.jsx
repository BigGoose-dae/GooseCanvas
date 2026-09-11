import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import Brand from "./Brand";
import StatusPanel from "./StatusPanel";
import ThemeToggle from "./ThemeToggle";
import LanguageToggle from "./LanguageToggle";
import { useLanguage } from "../i18n/LanguageContext";

export default function Projects() {
  const navigate = useNavigate();
  const { t } = useLanguage();
  const [items, setItems] = useState([]);
  const [status, setStatus] = useState(null);
  const [name, setName] = useState("");
  const [error, setError] = useState("");

  const load = async () => {
    try {
      const [appStatus, workspaces] = await Promise.all([
        api.status(),
        api.workspaces(),
      ]);
      setStatus(appStatus);
      setItems(workspaces);
    } catch (loadError) {
      setError(loadError.message);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const create = async (event) => {
    event.preventDefault();
    try {
      setError("");
      const item = await api.createWorkspace({
        name: name.trim() || t("untitledProject"),
      });
      navigate(`/canvas/${item.id}`);
    } catch (createError) {
      setError(createError.message);
    }
  };

  const remove = async (event, id) => {
    event.stopPropagation();
    if (!confirm(t("confirmDeleteProject"))) return;
    await api.deleteWorkspace(id);
    load();
  };

  const focusNewProject = () =>
    document.querySelector(".new-project-input")?.focus();

  return (
    <main className="projects-page">
      <header className="topbar home-topbar">
        <Brand app={status?.app} />
        <nav className="top-links">
          <button onClick={() => navigate("/settings")}>{t("settings")}</button>
          <button onClick={() => navigate("/settings/models")}>{t("modelManagement")}</button>
          <a
            href={status?.app?.repositoryUrl || "https://github.com"}
            target="_blank"
            rel="noreferrer"
          >
            GitHub <span aria-hidden="true">↗</span>
          </a>
          <ThemeToggle />
          <LanguageToggle />
        </nav>
      </header>

      <div className="home-shell">
        <section className="home-hero">
          <div className="hero-copy">
            <div className="eyebrow">
              <i /> {t("homeEyebrow")}
            </div>
            <h1>
              {t("homeTitleLine1")}
              <br />
              {t("homeTitleLine2")}
            </h1>
            <p>
              {t("homeDescription")}
            </p>
            <div className="hero-actions">
              <button
                type="button"
                className="hero-primary"
                onClick={focusNewProject}
              >
                {t("startCreating")} <span>→</span>
              </button>
              <span>{t("mediaTypes")}</span>
            </div>
          </div>

          <div className="hero-canvas" aria-hidden="true">
            <div className="hero-canvas-grid" />
            <div className="hero-flow-line line-one" />
            <div className="hero-flow-line line-two" />
            <div className="hero-node hero-node-text">
              <small>TEXT</small>
              <span>{t("heroPrompt")}</span>
              <b>01</b>
            </div>
            <div className="hero-node hero-node-image">
              <small>IMAGE</small>
              <div>
                <i />
                <i />
                <i />
              </div>
              <b>02</b>
            </div>
            <div className="hero-node hero-node-video">
              <small>VIDEO</small>
              <span>▶</span>
              <b>03</b>
            </div>
            <div className="hero-canvas-caption">
              <span>∞</span> {t("infiniteCanvas")}
            </div>
          </div>
        </section>

        <StatusPanel status={status} />

        <section className="project-section">
          <div className="section-title">
            <div>
              <span className="section-kicker">{t("yourWorkspaces")}</span>
              <h2>{t("recentProjects")}</h2>
              <p>
                {items.length
                  ? t("workspaceCount", { count: items.length })
                  : t("startBlank")}
              </p>
            </div>
            <form onSubmit={create}>
              <input
                className="new-project-input"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder={t("projectPlaceholder")}
                aria-label={t("projectName")}
              />
              <button type="submit">
                <span>＋</span> {t("newCanvas")}
              </button>
            </form>
          </div>
          {error && <div className="error-banner">{error}</div>}
          <div className="project-grid">
            {items.map((item, index) => (
              <article
                className="project-card"
                style={{ "--card-index": index }}
                key={item.id}
                onClick={() => navigate(`/canvas/${item.id}`)}
              >
                <div className="project-cover">
                  <div className="project-mini-grid" />
                  <div className="project-mini-node node-a" />
                  <div className="project-mini-node node-b" />
                  <div className="project-mini-node node-c" />
                  <svg
                    viewBox="0 0 320 170"
                    preserveAspectRatio="none"
                    aria-hidden="true"
                  >
                    <path d="M72 98 C118 98 120 58 170 58 S220 111 266 111" />
                  </svg>
                  <span>{String(index + 1).padStart(2, "0")}</span>
                </div>
                <div className="project-info">
                  <div>
                    <h3>{item.name}</h3>
                    <p>{item.description || t("continueCanvas")}</p>
                  </div>
                  <button
                    className="icon-button"
                    aria-label={t("deleteProjectLabel", { name: item.name })}
                    onClick={(event) => remove(event, item.id)}
                  >
                    ×
                  </button>
                </div>
              </article>
            ))}
            {!items.length && (
              <button className="empty-state" onClick={focusNewProject}>
                <span>＋</span>
                <strong>{t("createFirstProject")}</strong>
                <small>{t("enterCanvas")}</small>
              </button>
            )}
          </div>
        </section>

        <footer className="home-footer">
          <span>Goose Canvas</span>
          <span>Open source · Local first</span>
        </footer>
      </div>
    </main>
  );
}
