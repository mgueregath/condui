import { useState, useEffect, useRef } from "react";
import { FaDatabase, FaTable, FaChevronDown, FaChevronRight, FaPlay, FaKey } from "react-icons/fa";
import { LuRefreshCw } from "react-icons/lu";
import {
  DbConnect, DbDisconnect, DbListDatabases, DbListSchemas,
  DbListTables, DbGetColumns, DbQuery,
} from "../../bindings/ssh-gui/app";
import { useTranslation } from "react-i18next";

const DB_TYPE_COLOR = {
  postgresql: "#3b82f6", timescaledb: "#3b82f6",
  mysql: "#f97316", mariadb: "#c084fc",
};

function typeColor(dbType) {
  return DB_TYPE_COLOR[dbType?.toLowerCase()] || "#818cf8";
}

function normalizeType(name) {
  const l = (name || "").toLowerCase();
  if (l.includes("postgres") || l.includes("timescale")) return "postgresql";
  if (l.includes("mariadb")) return "mariadb";
  if (l.includes("mysql")) return "mysql";
  return l;
}

// ─── Connect form ─────────────────────────────────────────────────────────────
function ConnectForm({ dbType, port, sessionId, onConnected }) {
  const { t } = useTranslation();
  const nt = normalizeType(dbType);
  const defaultUser = nt === "postgresql" ? "postgres" : "root";
  const [user, setUser] = useState(defaultUser);
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const color = typeColor(nt);

  const submit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      const connId = await DbConnect(sessionId, dbType, port, user, password);
      onConnected(connId, nt);
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ flex: 1, display: "flex", alignItems: "center", justifyContent: "center", padding: "32px" }}>
      <div style={{
        width: "340px",
        background: "var(--bg-surface)",
        border: "1px solid var(--border-subtle)",
        borderTop: `3px solid ${color}`,
        borderRadius: "10px",
        padding: "24px",
        boxShadow: "0 8px 32px rgba(0,0,0,0.3)",
      }}>
        <div style={{ display: "flex", alignItems: "center", gap: "10px", marginBottom: "20px" }}>
          <FaDatabase style={{ color, fontSize: 20 }} />
          <div>
            <div style={{ fontWeight: "700", fontSize: "14px", color: "var(--text-primary)" }}>{dbType}</div>
            <div style={{ fontFamily: "var(--font-mono)", fontSize: "11px", color: "var(--text-muted)" }}>127.0.0.1:{port}</div>
          </div>
        </div>
        <form onSubmit={submit}>
          <label className="db-form-label">{t("database.user")}</label>
          <input className="db-form-input" value={user} onChange={e => setUser(e.target.value)} autoFocus />
          <label className="db-form-label">{t("database.password")}</label>
          <input className="db-form-input" type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder={t("database.passwordOptional")} />
          {error && <div className="db-form-error">{error}</div>}
          <button className="db-form-btn primary" type="submit" disabled={loading} style={{ width: "100%", marginTop: "16px" }}>
            {loading ? t("database.connecting") : t("database.connect")}
          </button>
        </form>
      </div>
    </div>
  );
}

// ─── Result grid ──────────────────────────────────────────────────────────────
function ResultGrid({ result }) {
  const { t } = useTranslation();
  if (!result) return <div className="db-result-empty">{t("database.runQueryHint")}</div>;
  if (result.error) return <div className="db-result-error">{result.error}</div>;
  if (!result.columns?.length) return <div className="db-result-empty">{t("database.noResults")}</div>;
  return (
    <div className="db-grid-wrap">
      <div className="db-grid-status">{t("database.rows", { count: result.rowCount })}</div>
      <div className="db-grid-scroll">
        <table className="db-grid">
          <thead>
            <tr>{result.columns.map((c, i) => <th key={i}>{c}</th>)}</tr>
          </thead>
          <tbody>
            {result.rows?.map((row, ri) => (
              <tr key={ri}>{row.map((cell, ci) => <td key={ci}>{cell ?? <span style={{opacity:.4}}>NULL</span>}</td>)}</tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ─── Structure view ───────────────────────────────────────────────────────────
function StructureView({ columns }) {
  const { t } = useTranslation();
  if (!columns?.length) return <div className="db-result-empty">{t("database.selectTable")}</div>;
  return (
    <div className="db-grid-scroll" style={{ padding: "8px 0" }}>
      <table className="db-grid">
        <thead>
          <tr><th>{t("database.column")}</th><th>{t("database.type")}</th><th>{t("database.nullable")}</th><th>{t("database.default")}</th><th>{t("database.key")}</th></tr>
        </thead>
        <tbody>
          {columns.map((c, i) => (
            <tr key={i}>
              <td style={{ fontWeight: c.key === "PK" || c.key === "PRI" ? 600 : 400 }}>
                {(c.key === "PK" || c.key === "PRI") && <FaKey style={{ fontSize: 9, color: "#fbbf24", marginRight: 4 }} />}
                {c.name}
              </td>
              <td style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{c.dataType}</td>
              <td style={{ color: c.nullable ? "var(--text-muted)" : "var(--green)" }}>{c.nullable ? t("common.yes") : t("common.no")}</td>
              <td style={{ fontFamily: "var(--font-mono)", fontSize: 11, color: "var(--text-muted)" }}>{c.default || "—"}</td>
              <td>{c.key && c.key !== "PRI" ? <span className="db-key-badge">{c.key}</span> : c.key === "PRI" ? <span className="db-key-badge pk">PK</span> : null}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Main explorer ────────────────────────────────────────────────────────────
export default function DbExplorer({ sessionId, dbType, port }) {
  const { t } = useTranslation();
  const [connId, setConnId]           = useState(null);
  const [ntType, setNtType]           = useState("");
  const [databases, setDatabases]     = useState([]);
  const [expandedDbs, setExpandedDbs] = useState({});
  const [schemas, setSchemas]         = useState({});
  const [expandedSch, setExpandedSch] = useState({});
  const [tables, setTables]           = useState({});
  const [selected, setSelected]       = useState(null);
  const [columns, setColumns]         = useState([]);
  const [tableData, setTableData]     = useState(null);
  const [currentDb, setCurrentDb]     = useState("");
  const [sql, setSql]                 = useState(() => t("database.sqlPlaceholder"));
  const [sqlResult, setSqlResult]     = useState(null);
  const [loading, setLoading]         = useState(false);
  const [activeTab, setActiveTab]     = useState("sql");
  const textareaRef = useRef(null);

  useEffect(() => {
    if (!connId) return;
    DbListDatabases(connId).then(dbs => setDatabases(dbs || [])).catch(() => {});
  }, [connId]);

  const disconnect = () => {
    if (connId) DbDisconnect(connId).catch(() => {});
    setConnId(null);
    setDatabases([]);
    setExpandedDbs({});
    setSchemas({});
    setTables({});
    setSelected(null);
    setColumns([]);
    setTableData(null);
    setSqlResult(null);
  };

  const toggleDb = async (db) => {
    const isOpen = !expandedDbs[db];
    setExpandedDbs(prev => ({ ...prev, [db]: isOpen }));
    if (!isOpen) return;
    if (ntType === "postgresql") {
      if (!schemas[db]) {
        const s = await DbListSchemas(connId, db).catch(() => []);
        setSchemas(prev => ({ ...prev, [db]: s || [] }));
      }
    } else {
      const key = `${db}\0`;
      if (!tables[key]) {
        const t = await DbListTables(connId, db, "").catch(() => []);
        setTables(prev => ({ ...prev, [key]: t || [] }));
      }
    }
  };

  const toggleSchema = async (db, schema) => {
    const key = `${db}\0${schema}`;
    const isOpen = !expandedSch[key];
    setExpandedSch(prev => ({ ...prev, [key]: isOpen }));
    if (!isOpen) return;
    if (!tables[key]) {
      const t = await DbListTables(connId, db, schema).catch(() => []);
      setTables(prev => ({ ...prev, [key]: t || [] }));
    }
  };

  const selectTable = async (db, schema, table) => {
    setSelected({ db, schema, table });
    setCurrentDb(db);
    const qualifier = schema ? `"${schema}"."${table}"` : `\`${table}\``;
    setSql(`SELECT * FROM ${qualifier} LIMIT 100;`);
    setActiveTab("data");
    setLoading(true);
    const [cols, data] = await Promise.all([
      DbGetColumns(connId, db, schema, table).catch(() => []),
      DbQuery(connId, db, `SELECT * FROM ${qualifier} LIMIT 200`).catch(e => ({ error: String(e), columns: [], rows: [] })),
    ]);
    setColumns(cols || []);
    setTableData(data);
    setLoading(false);
  };

  const runSql = async () => {
    if (!sql.trim() || !currentDb) return;
    setLoading(true);
    const r = await DbQuery(connId, currentDb, sql).catch(e => ({ error: String(e), columns: [], rows: [] }));
    setSqlResult(r);
    setActiveTab("sql");
    setLoading(false);
  };

  const handleKeyDown = (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
      e.preventDefault();
      runSql();
    }
  };

  if (!connId) {
    return (
      <div className="db-explorer-tab">
        <ConnectForm dbType={dbType} port={port} sessionId={sessionId} onConnected={(id, nt) => { setConnId(id); setNtType(nt); }} />
      </div>
    );
  }

  const color = typeColor(ntType);

  return (
    <div className="db-explorer-tab">
      {/* Header */}
      <div className="db-explorer-header" style={{ borderBottom: `2px solid ${color}33` }}>
        <FaDatabase style={{ color, fontSize: 14, flexShrink: 0 }} />
        <span className="db-explorer-title">{dbType}</span>
        <span className="db-explorer-port">:{port}</span>
        {currentDb && <span className="db-explorer-db">/{currentDb}</span>}
        <div style={{ flex: 1 }} />
        <button className="db-explorer-btn" title={t("database.reloadDatabases")} onClick={() => DbListDatabases(connId).then(dbs => setDatabases(dbs || []))}>
          <LuRefreshCw />
        </button>
        <button className="db-explorer-btn disconnect" onClick={disconnect} title={t("database.disconnect")}>⏏ {t("database.disconnect")}</button>
      </div>

      <div className="db-explorer-body">
        {/* Tree */}
        <div className="db-tree">
          <div className="db-tree-header">{t("database.databases")}</div>
          {databases.map(db => (
            <div key={db}>
              <div className="db-tree-item db-level" onClick={() => toggleDb(db)}>
                <span className="db-tree-arrow">{expandedDbs[db] ? <FaChevronDown /> : <FaChevronRight />}</span>
                <FaDatabase style={{ fontSize: 11, color, opacity: .8 }} />
                <span>{db}</span>
              </div>
              {expandedDbs[db] && ntType === "postgresql" && (schemas[db] || []).map(schema => (
                <div key={schema}>
                  <div className="db-tree-item schema-level" onClick={() => toggleSchema(db, schema)}>
                    <span className="db-tree-arrow">{expandedSch[`${db}\0${schema}`] ? <FaChevronDown /> : <FaChevronRight />}</span>
                    <span className="db-tree-schema-icon">⬡</span>
                    <span>{schema}</span>
                  </div>
                  {expandedSch[`${db}\0${schema}`] && (tables[`${db}\0${schema}`] || []).map(t => (
                    <div
                      key={t}
                      className={"db-tree-item table-level" + (selected?.table === t && selected?.db === db ? " active" : "")}
                      onClick={() => { setCurrentDb(db); selectTable(db, schema, t); }}
                    >
                      <FaTable style={{ fontSize: 10, color: "var(--text-muted)" }} />
                      <span>{t}</span>
                    </div>
                  ))}
                </div>
              ))}
              {expandedDbs[db] && ntType !== "postgresql" && (tables[`${db}\0`] || []).map(t => (
                <div
                  key={t}
                  className={"db-tree-item table-level" + (selected?.table === t && selected?.db === db ? " active" : "")}
                  onClick={() => { setCurrentDb(db); selectTable(db, "", t); }}
                >
                  <FaTable style={{ fontSize: 10, color: "var(--text-muted)" }} />
                  <span>{t}</span>
                </div>
              ))}
            </div>
          ))}
        </div>

        {/* Workspace */}
        <div className="db-workspace">
          {/* SQL editor */}
          <div className="db-sql-pane">
            <div className="db-sql-toolbar">
              <select
                className="db-db-select"
                value={currentDb}
                onChange={e => setCurrentDb(e.target.value)}
              >
                <option value="">{t("database.selectDatabase")}</option>
                {databases.map(d => <option key={d} value={d}>{d}</option>)}
              </select>
              <div style={{ flex: 1 }} />
              <span style={{ fontSize: 10, color: "var(--text-muted)" }}>{t("database.runShortcut")}</span>
              <button className="db-run-btn" onClick={runSql} disabled={loading || !currentDb}>
                <FaPlay style={{ fontSize: 10 }} /> {loading ? "…" : t("database.run")}
              </button>
            </div>
            <textarea
              ref={textareaRef}
              className="db-sql-editor"
              value={sql}
              onChange={e => setSql(e.target.value)}
              onKeyDown={handleKeyDown}
              spellCheck={false}
            />
          </div>

          {/* Result tabs */}
          <div className="db-result-pane">
            <div className="db-result-tabs">
              <button className={"db-result-tab" + (activeTab === "sql" ? " active" : "")} onClick={() => setActiveTab("sql")}>
                {t("database.sqlResult")} {sqlResult && <span className="db-tab-count">{sqlResult.rowCount ?? 0}</span>}
              </button>
              {selected && <>
                <button className={"db-result-tab" + (activeTab === "data" ? " active" : "")} onClick={() => setActiveTab("data")}>
                  {t("database.data", { table: selected.table })} {tableData && <span className="db-tab-count">{tableData.rowCount ?? 0}</span>}
                </button>
                <button className={"db-result-tab" + (activeTab === "structure" ? " active" : "")} onClick={() => setActiveTab("structure")}>
                  {t("database.structure")}
                </button>
              </>}
            </div>
            <div className="db-result-body">
              {activeTab === "sql"       && <ResultGrid result={sqlResult} />}
              {activeTab === "data"      && <ResultGrid result={tableData} />}
              {activeTab === "structure" && <StructureView columns={columns} />}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
