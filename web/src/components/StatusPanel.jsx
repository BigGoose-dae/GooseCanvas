export default function StatusPanel({ status }) {
  if (!status || status.ready) return null;
  return (
    <div className="status-panel">
      <div>
        <strong>还差一步即可开始</strong>
        <span>前往设置页面，连接生成模型与素材存储。</span>
      </div>
      <a href="/settings">打开创作设置 →</a>
      <div className="status-tags">
        {(status.missing || []).map((item) => (
          <code key={item}>
            {{
              VOLCENGINE_API_KEY: "模型密钥",
              TOS_BUCKET: "存储桶",
              TOS_ACCESS_KEY: "存储访问密钥",
              TOS_SECRET_KEY: "存储私有密钥",
            }[item] || item}
          </code>
        ))}
        {status.storage?.status === "error" && <code>存储连接失败</code>}
      </div>
      {status.storage?.message && <small>{status.storage.message}</small>}
    </div>
  );
}
