import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import Brand from "./Brand";
import StatusPanel from "./StatusPanel";
import ThemeToggle from "./ThemeToggle";

export default function Projects() {
  const navigate = useNavigate();
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
        name: name.trim() || "未命名项目",
      });
      navigate(`/canvas/${item.id}`);
    } catch (createError) {
      setError(createError.message);
    }
  };

  const remove = async (event, id) => {
    event.stopPropagation();
    if (!confirm("删除这个项目？")) return;
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
          <button onClick={() => navigate("/settings")}>设置</button>
          <button onClick={() => navigate("/settings/models")}>模型管理</button>
          <a
            href={status?.app?.repositoryUrl || "https://github.com"}
            target="_blank"
            rel="noreferrer"
          >
            GitHub <span aria-hidden="true">↗</span>
          </a>
          <ThemeToggle />
        </nav>
      </header>

      <div className="home-shell">
        <section className="home-hero">
          <div className="hero-copy">
            <div className="eyebrow">
              <i /> LOCAL-FIRST CREATIVE CANVAS
            </div>
            <h1>
              让灵感，
              <br />
              自由生长。
            </h1>
            <p>
              一个轻量、开放的 AI
              创作画布。连接文字、图像与视频，在属于你的工作流里持续创作。
            </p>
            <div className="hero-actions">
              <button
                type="button"
                className="hero-primary"
                onClick={focusNewProject}
              >
                开始创作 <span>→</span>
              </button>
              <span>文字 · 图像 · 视频</span>
            </div>
          </div>

          <div className="hero-canvas" aria-hidden="true">
            <div className="hero-canvas-grid" />
            <div className="hero-flow-line line-one" />
            <div className="hero-flow-line line-two" />
            <div className="hero-node hero-node-text">
              <small>TEXT</small>
              <span>想象一座漂浮在云端的城市</span>
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
              <span>∞</span> 无限画布 · 自由连接
            </div>
          </div>
        </section>

        <StatusPanel status={status} />

        <section className="project-section">
          <div className="section-title">
            <div>
              <span className="section-kicker">YOUR WORKSPACES</span>
              <h2>最近项目</h2>
              <p>
                {items.length
                  ? `${items.length} 个创作空间`
                  : "从一张空白画布开始"}
              </p>
            </div>
            <form onSubmit={create}>
              <input
                className="new-project-input"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="给新项目起个名字"
                aria-label="项目名称"
              />
              <button type="submit">
                <span>＋</span> 新建画布
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
                    <p>{item.description || "继续你的画布创作"}</p>
                  </div>
                  <button
                    className="icon-button"
                    aria-label={`删除${item.name}`}
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
                <strong>创建第一个项目</strong>
                <small>进入你的无限画布</small>
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
