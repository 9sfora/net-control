import { FormEvent, ReactNode, useCallback, useEffect, useState } from "react";
import {
  Activity,
  Boxes,
  ChevronRight,
  CircleArrowUp,
  CircleGauge,
  Command,
  Cpu,
  Download,
  LogOut,
  Menu,
  PackageOpen,
  Plus,
  RefreshCw,
  Router,
  Search,
  Star,
  Trash2,
  Upload,
  Wifi,
  X,
} from "lucide-react";
import {
  Navigate,
  NavLink,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from "react-router-dom";
import {
  api,
  ApiError,
  auth,
  downloadFile,
  formatDate,
  label,
  type Principal,
} from "./api";

type Envelope<T> = { data: T };
type ListEnvelope<T> = Envelope<T[]> & {
  pagination?: { count: number; limit: number; offset: number };
};
type DashboardData = {
  routers: {
    total: number;
    online: number;
    stale: number;
    offline: number;
    never_connected: number;
    blocked: number;
  };
  commands: {
    pending: number;
    waiting_device: number;
    processing: number;
    retry_scheduled: number;
    failed: number;
    expired: number;
    completed_last_24_hours: number;
  };
  recent_connections: {
    router_id: string;
    router_name: string;
    serial_number: string;
    ip_address?: string;
    last_seen_at: string;
  }[];
  recent_activity: CommandItem[];
  recent_failures: CommandItem[];
  generated_at: string;
};
type RouterItem = {
  id: string;
  name: string;
  manufacturer: string;
  router_model_name?: string;
  serial_number: string;
  connectivity_status: string;
  desired_status: string;
  last_seen_at?: string;
  last_ip_address?: string;
  software_version?: string;
};
type UnknownRouter = {
  id: string;
  manufacturer: string;
  oui: string;
  product_class: string;
  serial_number: string;
  last_ip_address?: string;
  hardware_version?: string;
  software_version?: string;
  last_seen_at: string;
  inform_count: number;
};
type CommandItem = {
  id?: string;
  command_id?: string;
  router_id: string;
  router_name: string;
  type: string;
  status: string;
  attempts_count?: number;
  created_at: string;
  error_message?: string;
};
type Manufacturer = {
  id: string;
  name: string;
  description?: string;
  active: boolean;
};
type HardwareVersion = { id: string; version: string; active: boolean };
type Model = {
  id: string;
  manufacturer_id: string;
  name: string;
  manufacturer_name: string;
  manufacturer_model_name?: string;
  protocol: string;
  product_class?: string;
  oui?: string;
  description?: string;
  supports_wifi_2_4: boolean;
  supports_wifi_5: boolean;
  supported_auth_modes: string[];
  active: boolean;
  hardware_versions?: HardwareVersion[];
};
type Firmware = {
  id: string;
  router_model_id: string;
  hardware_version_id?: string;
  name: string;
  version: string;
  router_model_name: string;
  hardware_version?: string;
  description?: string;
  release_notes?: string;
  status: string;
  is_default: boolean;
  active: boolean;
  file_name?: string;
  file_size?: number;
  updated_at: string;
};

const statusTone = (status: string) =>
  ["ONLINE", "ACTIVE", "READY", "COMPLETED", "PUBLISHED"].includes(status)
    ? "good"
    : ["FAILED", "OFFLINE", "EXPIRED", "BLOCKED"].includes(status)
      ? "bad"
      : "warn";
function Badge({ value }: { value: string }) {
  return (
    <span className={`badge ${statusTone(value)}`}>
      <i />
      {label[value] || value}
    </span>
  );
}

function App() {
  const [user, setUser] = useState<Principal | null>(null);
  const [checking, setChecking] = useState(Boolean(auth.token()));
  useEffect(() => {
    if (!auth.token()) return;
    api<{ user: Principal }>("/api/v1/auth/me")
      .then((r) => setUser(r.user))
      .catch(() => auth.clear())
      .finally(() => setChecking(false));
  }, []);
  useEffect(() => {
    const logout = () => setUser(null);
    window.addEventListener("netcontrol:session-expired", logout);
    return () =>
      window.removeEventListener("netcontrol:session-expired", logout);
  }, []);
  if (checking)
    return (
      <div className="app-loader">
        <span className="brand-mark">
          <Wifi />
        </span>
      </div>
    );
  return user ? (
    <Shell
      user={user}
      onLogout={() => {
        auth.clear();
        setUser(null);
      }}
    />
  ) : (
    <Login onLogin={setUser} />
  );
}

function Login({ onLogin }: { onLogin: (user: Principal) => void }) {
  const [tenant, setTenant] = useState("netcontrol"),
    [email, setEmail] = useState(""),
    [password, setPassword] = useState("");
  const [error, setError] = useState(""),
    [loading, setLoading] = useState(false);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setLoading(true);
    try {
      const session = await api<{ access_token: string }>(
        "/api/v1/auth/login",
        { method: "POST", body: JSON.stringify({ tenant, email, password }) },
      );
      auth.set(session.access_token);
      const me = await api<{ user: Principal }>("/api/v1/auth/me");
      onLogin(me.user);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Não foi possível entrar.");
    } finally {
      setLoading(false);
    }
  }
  return (
    <main className="login-page">
      <section className="login-card">
        <div className="login-brand">
          <span className="brand-mark">
            <Wifi size={20} />
          </span>
          <span>
            net<span>control</span>
          </span>
        </div>
        <div className="login-copy">
          <p className="eyebrow">Bem-vindo de volta</p>
          <h1>Acesse sua conta</h1>
          <p>Gerencie seus equipamentos de rede em um só lugar.</p>
        </div>
        <form onSubmit={submit}>
          <Field label="Tenant">
            <input
              value={tenant}
              onChange={(e) => setTenant(e.target.value)}
              placeholder="netcontrol"
              autoComplete="organization"
              required
            />
          </Field>
          <Field label="E-mail">
            <input
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              type="email"
              placeholder="seu@email.com"
              autoComplete="email"
              required
              autoFocus
            />
          </Field>
          <Field label="Senha">
            <input
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              type="password"
              placeholder="Sua senha"
              autoComplete="current-password"
              required
            />
          </Field>
          {error && <div className="form-error">{error}</div>}
          <button className="button primary login-button" disabled={loading}>
            {loading ? (
              <>
                <RefreshCw className="spin" size={16} /> Entrando...
              </>
            ) : (
              "Entrar"
            )}
          </button>
        </form>
        <p className="login-foot">Acesso restrito a usuários autorizados</p>
      </section>
    </main>
  );
}

const nav = [
  {
    to: "/dashboard",
    label: "Dashboard",
    icon: CircleGauge,
    permission: "acs.dashboard.read",
  },
  {
    to: "/routers",
    label: "Roteadores",
    icon: Router,
    permission: "acs.router.read",
  },
  {
    to: "/discovery",
    label: "Descoberta",
    icon: Activity,
    permission: "acs.unknown_router.read",
  },
  {
    to: "/commands",
    label: "Comandos",
    icon: Command,
    permission: "acs.command.read",
  },
  {
    to: "/catalog",
    label: "Catálogo",
    icon: Boxes,
    permission: "acs.catalog.read",
  },
  {
    to: "/firmwares",
    label: "Firmwares",
    icon: Cpu,
    permission: "acs.firmware.read",
  },
];
function Shell({ user, onLogout }: { user: Principal; onLogout: () => void }) {
  const [open, setOpen] = useState(false);
  const location = useLocation();
  useEffect(() => setOpen(false), [location.pathname]);
  const visible = nav.filter((item) =>
    user.permissions.includes(item.permission),
  );
  const first = visible[0]?.to || "/empty";
  return (
    <div className="shell">
      <aside className={open ? "sidebar open" : "sidebar"}>
        <div className="sidebar-head">
          <div className="login-brand">
            <span className="brand-mark">
              <Wifi size={18} />
            </span>
            <span>
              net<span>control</span>
            </span>
          </div>
          <button
            className="icon-button close-menu"
            onClick={() => setOpen(false)}
          >
            <X />
          </button>
        </div>
        <nav>
          {visible.map(({ to, label: text, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) => (isActive ? "active" : "")}
            >
              <Icon size={19} />
              <span>{text}</span>
            </NavLink>
          ))}
        </nav>
        <div className="account">
          <div className="avatar">{user.name.slice(0, 2).toUpperCase()}</div>
          <div>
            <strong>{user.name}</strong>
            <span>{user.email}</span>
          </div>
          <button title="Sair" onClick={onLogout}>
            <LogOut size={18} />
          </button>
        </div>
      </aside>
      {open && <div className="backdrop" onClick={() => setOpen(false)} />}
      <div className="content">
        <header>
          <button
            className="icon-button menu-button"
            onClick={() => setOpen(true)}
          >
            <Menu />
          </button>
          <div>
            <span>NetControl</span>
            <small>Módulo ACS</small>
          </div>
          <div className="header-status">
            <i /> Sistema operacional
          </div>
        </header>
        <Routes>
          <Route path="/" element={<Navigate to={first} replace />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/routers" element={<Routers />} />
          <Route
            path="/discovery"
            element={
              <Discovery
                canManageCatalog={user.permissions.includes(
                  "acs.catalog.manage",
                )}
                canPromote={user.permissions.includes(
                  "acs.unknown_router.configure",
                )}
              />
            }
          />
          <Route
            path="/commands"
            element={
              <Commands
                canCancel={user.permissions.includes("acs.command.cancel")}
              />
            }
          />
          <Route
            path="/catalog"
            element={
              <Catalog
                canManage={user.permissions.includes("acs.catalog.manage")}
              />
            }
          />
          <Route
            path="/firmwares"
            element={
              <Firmwares
                canManage={user.permissions.includes("acs.firmware.create")}
                canUpload={user.permissions.includes("acs.firmware.upload")}
              />
            }
          />
          <Route path="*" element={<Navigate to={first} replace />} />
        </Routes>
      </div>
    </div>
  );
}

function useData<T>(path: string) {
  const [data, setData] = useState<T | null>(null),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(true),
    [tick, setTick] = useState(0);
  useEffect(() => {
    let active = true;
    setLoading(true);
    setError("");
    api<T>(path)
      .then((v) => active && setData(v))
      .catch((e) => active && setError(e.message))
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [path, tick]);
  return { data, error, loading, reload: () => setTick((v) => v + 1) };
}
function Page({
  title,
  description,
  action,
  children,
}: {
  title: string;
  description: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <main className="page">
      <div className="page-title">
        <div>
          <h1>{title}</h1>
          <p>{description}</p>
        </div>
        {action}
      </div>
      {children}
    </main>
  );
}
function State({
  loading,
  error,
  empty,
  reload,
}: {
  loading: boolean;
  error: string;
  empty?: boolean;
  reload: () => void;
}) {
  if (loading)
    return (
      <div className="state">
        <RefreshCw className="spin" />
        <p>Carregando dados...</p>
      </div>
    );
  if (error)
    return (
      <div className="state">
        <p>{error}</p>
        <button className="button" onClick={reload}>
          Tentar novamente
        </button>
      </div>
    );
  if (empty)
    return (
      <div className="state">
        <PackageOpen />
        <p>Nenhum registro encontrado.</p>
      </div>
    );
  return null;
}

function Dashboard() {
  const { data: envelope, ...state } = useData<Envelope<DashboardData>>(
    "/api/v1/acs/dashboard",
  );
  const d = envelope?.data;
  return (
    <Page
      title="Visão geral"
      description="Acompanhe a operação da sua rede em tempo real."
      action={
        <button className="button" onClick={state.reload}>
          <RefreshCw size={16} /> Atualizar
        </button>
      }
    >
      <State {...state} reload={state.reload} />
      {d && (
        <>
          <div className="stats">
            <Stat
              icon={<Router />}
              title="Total de roteadores"
              value={d.routers.total}
              note={`${d.routers.never_connected} nunca conectados`}
            />
            <Stat
              icon={<Wifi />}
              title="Roteadores online"
              value={d.routers.online}
              note={`${d.routers.stale} com sinal instável`}
              tone="green"
            />
            <Stat
              icon={<Activity />}
              title="Roteadores offline"
              value={d.routers.offline}
              note={`${d.routers.blocked} bloqueados`}
              tone="red"
            />
            <Stat
              icon={<Command />}
              title="Comandos na fila"
              value={
                d.commands.pending +
                d.commands.waiting_device +
                d.commands.processing +
                d.commands.retry_scheduled
              }
              note={`${d.commands.completed_last_24_hours} concluídos em 24h`}
              tone="blue"
            />
          </div>
          <div className="grid-two">
            <Panel title="Conexões recentes" link="/routers">
              <div className="activity-list">
                {d.recent_connections.length ? (
                  d.recent_connections.map((item) => (
                    <div className="activity-row" key={item.router_id}>
                      <span className="device-icon">
                        <Router size={18} />
                      </span>
                      <div>
                        <strong>{item.router_name}</strong>
                        <span>
                          {item.serial_number} ·{" "}
                          {item.ip_address || "IP indisponível"}
                        </span>
                      </div>
                      <time>{formatDate(item.last_seen_at)}</time>
                    </div>
                  ))
                ) : (
                  <EmptyInline />
                )}
              </div>
            </Panel>
            <Panel title="Atividade de comandos" link="/commands">
              <div className="activity-list">
                {d.recent_activity.length ? (
                  d.recent_activity.map((item, i) => (
                    <div className="activity-row" key={item.command_id || i}>
                      <span className="device-icon">
                        <Command size={18} />
                      </span>
                      <div>
                        <strong>{item.router_name}</strong>
                        <span>{label[item.type] || item.type}</span>
                      </div>
                      <Badge value={item.status} />
                    </div>
                  ))
                ) : (
                  <EmptyInline />
                )}
              </div>
            </Panel>
          </div>
        </>
      )}
    </Page>
  );
}
function Stat({
  icon,
  title,
  value,
  note,
  tone = "",
}: {
  icon: ReactNode;
  title: string;
  value: number;
  note: string;
  tone?: string;
}) {
  return (
    <article className={`stat ${tone}`}>
      <div className="stat-top">
        <span>{icon}</span>
        <small>{title}</small>
      </div>
      <strong>{value.toLocaleString("pt-BR")}</strong>
      <p>{note}</p>
    </article>
  );
}
function Panel({
  title,
  link,
  children,
}: {
  title: string;
  link: string;
  children: ReactNode;
}) {
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>{title}</h2>
        <NavLink to={link}>
          Ver todos <ChevronRight size={15} />
        </NavLink>
      </div>
      {children}
    </section>
  );
}
function EmptyInline() {
  return <div className="empty-inline">Nenhuma atividade recente.</div>;
}

function Routers() {
  const [search, setSearch] = useState(""),
    [status, setStatus] = useState("");
  const query = new URLSearchParams({ limit: "100" });
  if (search) query.set("search", search);
  if (status) query.set("connectivity_status", status);
  const state = useData<ListEnvelope<RouterItem>>(
    `/api/v1/acs/routers?${query}`,
  );
  const rows = state.data?.data || [];
  return (
    <Page
      title="Roteadores"
      description="Consulte e acompanhe os equipamentos configurados."
    >
      <Toolbar search={search} setSearch={setSearch}>
        <select value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">Todos os status</option>
          <option value="ONLINE">Online</option>
          <option value="STALE">Instável</option>
          <option value="OFFLINE">Offline</option>
          <option value="NEVER_CONNECTED">Nunca conectado</option>
        </select>
      </Toolbar>
      <State {...state} empty={!rows.length} reload={state.reload} />
      {rows.length > 0 && (
        <Table
          headers={[
            "Equipamento",
            "Modelo",
            "Serial",
            "Endereço IP",
            "Última conexão",
            "Status",
          ]}
        >
          {rows.map((r) => (
            <tr key={r.id}>
              <td>
                <strong>{r.name}</strong>
                <small>{r.manufacturer}</small>
              </td>
              <td>{r.router_model_name || "—"}</td>
              <td className="mono">{r.serial_number}</td>
              <td className="mono">{r.last_ip_address || "—"}</td>
              <td>{formatDate(r.last_seen_at)}</td>
              <td>
                <Badge value={r.connectivity_status} />
              </td>
            </tr>
          ))}
        </Table>
      )}
    </Page>
  );
}
function Discovery({
  canManageCatalog,
  canPromote,
}: {
  canManageCatalog: boolean;
  canPromote: boolean;
}) {
  const state = useData<ListEnvelope<UnknownRouter>>(
      "/api/v1/acs/unknown-routers?limit=100",
    ),
    modelsState = useData<Envelope<Model[]>>("/api/v1/acs/router-models"),
    manufacturersState = useData<Envelope<Manufacturer[]>>(
      "/api/v1/acs/manufacturers",
    );
  const rows = state.data?.data || [];
  const [modelSource, setModelSource] = useState<UnknownRouter | undefined>(),
    [promoting, setPromoting] = useState<UnknownRouter | undefined>();
  function modelDraft(router: UnknownRouter): Partial<Model> {
    const manufacturer = (manufacturersState.data?.data || []).find(
      (item) =>
        item.name.localeCompare(router.manufacturer, undefined, {
          sensitivity: "base",
        }) === 0,
    );
    return {
      manufacturer_id: manufacturer?.id || "",
      name: [router.manufacturer, router.product_class]
        .filter(Boolean)
        .join(" "),
      manufacturer_model_name: router.product_class || "",
      product_class: router.product_class || undefined,
      oui: router.oui || undefined,
      protocol: "TR-069",
      supports_wifi_2_4: false,
      supports_wifi_5: false,
      supported_auth_modes: [],
      hardware_versions: router.hardware_version
        ? [{ id: "", version: router.hardware_version, active: true }]
        : [],
      active: true,
      description: `Modelo identificado pelo equipamento ${router.serial_number}.`,
    };
  }
  return (
    <Page
      title="Descoberta"
      description="Equipamentos identificados pelo ACS que ainda precisam ser configurados."
      action={
        <button
          className="button"
          onClick={() => {
            state.reload();
            modelsState.reload();
            manufacturersState.reload();
          }}
        >
          <RefreshCw size={16} /> Atualizar
        </button>
      }
    >
      <State {...state} empty={!rows.length} reload={state.reload} />
      {rows.length > 0 && (
        <Table
          headers={[
            "Fabricante",
            "Product class",
            "Serial",
            "Hardware",
            "Último IP",
            "Informs",
            "Visto por último",
            "Ações",
          ]}
        >
          {rows.map((r) => (
            <tr key={r.id}>
              <td>
                <strong>{r.manufacturer || "Desconhecido"}</strong>
                <small>OUI {r.oui || "—"}</small>
              </td>
              <td>{r.product_class || "—"}</td>
              <td className="mono">{r.serial_number}</td>
              <td>{r.hardware_version || "—"}</td>
              <td className="mono">{r.last_ip_address || "—"}</td>
              <td>{r.inform_count}</td>
              <td>{formatDate(r.last_seen_at)}</td>
              <td>
                <div className="row-actions">
                  {canManageCatalog && (
                    <button
                      title="Cadastrar modelo"
                      onClick={() => setModelSource(r)}
                    >
                      <Boxes />
                    </button>
                  )}
                  {canPromote && (
                    <button
                      title="Promover roteador"
                      onClick={() => setPromoting(r)}
                    >
                      <Router />
                    </button>
                  )}
                </div>
              </td>
            </tr>
          ))}
        </Table>
      )}
      {modelSource && (
        <ModelModal
          item={null}
          initial={modelDraft(modelSource)}
          manufacturers={manufacturersState.data?.data || []}
          onClose={() => setModelSource(undefined)}
          onSaved={() => {
            setModelSource(undefined);
            modelsState.reload();
          }}
        />
      )}
      {promoting && (
        <PromoteRouterModal
          router={promoting}
          models={modelsState.data?.data || []}
          onClose={() => setPromoting(undefined)}
          onSaved={() => {
            setPromoting(undefined);
            state.reload();
          }}
        />
      )}
    </Page>
  );
}
function Commands({ canCancel }: { canCancel: boolean }) {
  const [status, setStatus] = useState("");
  const state = useData<ListEnvelope<CommandItem>>(
    `/api/v1/acs/commands?limit=100${status ? `&status=${status}` : ""}`,
  );
  const rows = state.data?.data || [];
  async function cancel(id?: string) {
    if (!id || !confirm("Cancelar este comando?")) return;
    try {
      await api(`/api/v1/acs/commands/${id}/cancel`, { method: "POST" });
      state.reload();
    } catch (e) {
      alert(e instanceof Error ? e.message : "Erro ao cancelar");
    }
  }
  return (
    <Page
      title="Comandos"
      description="Acompanhe o processamento e o histórico da fila."
    >
      <div className="toolbar">
        <div />
        <select value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">Todos os status</option>
          {[
            "PENDING",
            "WAITING_DEVICE",
            "PROCESSING",
            "RETRY_SCHEDULED",
            "COMPLETED",
            "FAILED",
            "CANCELED",
            "EXPIRED",
          ].map((s) => (
            <option key={s} value={s}>
              {label[s]}
            </option>
          ))}
        </select>
      </div>
      <State {...state} empty={!rows.length} reload={state.reload} />
      {rows.length > 0 && (
        <Table
          headers={[
            "Roteador",
            "Comando",
            "Criado em",
            "Tentativas",
            "Status",
            "",
          ]}
        >
          {rows.map((r) => (
            <tr key={r.id}>
              <td>
                <strong>{r.router_name}</strong>
              </td>
              <td>{label[r.type] || r.type}</td>
              <td>{formatDate(r.created_at)}</td>
              <td>{r.attempts_count ?? "—"}</td>
              <td>
                <Badge value={r.status} />
              </td>
              <td>
                {canCancel &&
                  ["PENDING", "WAITING_DEVICE", "RETRY_SCHEDULED"].includes(
                    r.status,
                  ) && (
                    <button
                      className="text-button danger"
                      onClick={() => cancel(r.id)}
                    >
                      Cancelar
                    </button>
                  )}
              </td>
            </tr>
          ))}
        </Table>
      )}
    </Page>
  );
}
function Catalog({ canManage }: { canManage: boolean }) {
  const [tab, setTab] = useState<"models" | "manufacturers">("models"),
    [modelModal, setModelModal] = useState<Model | null | undefined>(undefined),
    [manufacturerModal, setManufacturerModal] = useState<
      Manufacturer | null | undefined
    >(undefined);
  const modelsState = useData<Envelope<Model[]>>("/api/v1/acs/router-models"),
    manufacturersState = useData<Envelope<Manufacturer[]>>(
      "/api/v1/acs/manufacturers",
    );
  const state = tab === "models" ? modelsState : manufacturersState,
    rows = state.data?.data || [];
  function reload() {
    tab === "models" ? modelsState.reload() : manufacturersState.reload();
  }
  async function remove(
    kind: "model" | "manufacturer",
    id: string,
    name: string,
  ) {
    if (!confirm(`Excluir “${name}”?`)) return;
    try {
      await api(
        `/api/v1/acs/${kind === "model" ? "router-models" : "manufacturers"}/${id}`,
        { method: "DELETE" },
      );
      kind === "model" ? modelsState.reload() : manufacturersState.reload();
    } catch (e) {
      alert(e instanceof Error ? e.message : "Erro ao excluir");
    }
  }
  async function openModel(model: Model) {
    if (!canManage) return;
    try {
      const result = await api<Envelope<Model>>(
        `/api/v1/acs/router-models/${model.id}`,
      );
      setModelModal(result.data);
    } catch (e) {
      alert(e instanceof Error ? e.message : "Erro ao carregar modelo");
    }
  }
  const action = (
    <div className="page-actions">
      <button className="button" onClick={reload}>
        <RefreshCw size={16} /> Atualizar
      </button>
      {canManage && (
        <button
          className="button primary"
          onClick={() =>
            tab === "models" ? setModelModal(null) : setManufacturerModal(null)
          }
        >
          <Plus size={16} /> Adicionar{" "}
          {tab === "models" ? "modelo" : "fabricante"}
        </button>
      )}
    </div>
  );
  return (
    <Page
      title="Catálogo"
      description="Fabricantes e modelos homologados para sua operação."
      action={action}
    >
      <div className="tabs">
        <button
          className={tab === "models" ? "active" : ""}
          onClick={() => setTab("models")}
        >
          Modelos
        </button>
        <button
          className={tab === "manufacturers" ? "active" : ""}
          onClick={() => setTab("manufacturers")}
        >
          Fabricantes
        </button>
      </div>
      <State {...state} empty={!rows.length} reload={reload} />
      {rows.length > 0 &&
        (tab === "models" ? (
          <Table
            headers={[
              "Modelo",
              "Fabricante",
              "Protocolo",
              "Wi-Fi",
              "Product class",
              "Status",
              "Ações",
            ]}
          >
            {(rows as Model[]).map((m) => (
              <tr
                className={canManage ? "clickable-row" : ""}
                key={m.id}
                onClick={() => openModel(m)}
              >
                <td>
                  <strong>{m.name}</strong>
                  <small>{m.manufacturer_model_name || ""}</small>
                </td>
                <td>{m.manufacturer_name}</td>
                <td>{m.protocol}</td>
                <td>
                  {[
                    m.supports_wifi_2_4 && "2.4 GHz",
                    m.supports_wifi_5 && "5 GHz",
                  ]
                    .filter(Boolean)
                    .join(" · ") || "—"}
                </td>
                <td>{m.product_class || "—"}</td>
                <td>
                  <Badge value={m.active ? "ACTIVE" : "INACTIVE"} />
                </td>
                <td>
                  {canManage && (
                    <div
                      className="row-actions"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <button
                        className="danger"
                        title="Excluir"
                        onClick={() => remove("model", m.id, m.name)}
                      >
                        <Trash2 />
                      </button>
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </Table>
        ) : (
          <Table headers={["Fabricante", "Descrição", "Status", "Ações"]}>
            {(rows as Manufacturer[]).map((m) => (
              <tr
                className={canManage ? "clickable-row" : ""}
                key={m.id}
                onClick={() => canManage && setManufacturerModal(m)}
              >
                <td>
                  <strong>{m.name}</strong>
                </td>
                <td>{m.description || "—"}</td>
                <td>
                  <Badge value={m.active ? "ACTIVE" : "INACTIVE"} />
                </td>
                <td>
                  {canManage && (
                    <div
                      className="row-actions"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <button
                        className="danger"
                        title="Excluir"
                        onClick={() => remove("manufacturer", m.id, m.name)}
                      >
                        <Trash2 />
                      </button>
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </Table>
        ))}
      {manufacturerModal !== undefined && (
        <ManufacturerModal
          item={manufacturerModal}
          onClose={() => setManufacturerModal(undefined)}
          onSaved={() => {
            setManufacturerModal(undefined);
            manufacturersState.reload();
          }}
        />
      )}
      {modelModal !== undefined && (
        <ModelModal
          item={modelModal}
          manufacturers={manufacturersState.data?.data || []}
          onClose={() => setModelModal(undefined)}
          onSaved={() => {
            setModelModal(undefined);
            modelsState.reload();
          }}
        />
      )}
    </Page>
  );
}

function ManufacturerModal({
  item,
  onClose,
  onSaved,
}: {
  item: Manufacturer | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(item?.name || ""),
    [description, setDescription] = useState(item?.description || ""),
    [active, setActive] = useState(item?.active ?? true),
    [saving, setSaving] = useState(false),
    [error, setError] = useState("");
  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await api(
        item
          ? `/api/v1/acs/manufacturers/${item.id}`
          : "/api/v1/acs/manufacturers",
        {
          method: item ? "PUT" : "POST",
          body: JSON.stringify({
            name,
            description: description || null,
            active,
          }),
        },
      );
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível salvar.");
    } finally {
      setSaving(false);
    }
  }
  return (
    <CatalogModal
      title={item ? "Editar fabricante" : "Adicionar fabricante"}
      description="Informe os dados do fabricante."
      saving={saving}
      error={error}
      onClose={onClose}
      onSubmit={submit}
    >
      <Field label="Nome">
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={200}
          required
          autoFocus
        />
      </Field>
      <Field label="Descrição">
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={4}
        />
      </Field>
      <div className="check-row">
        <label>
          <input
            type="checkbox"
            checked={active}
            onChange={(e) => setActive(e.target.checked)}
          />{" "}
          Ativo
        </label>
      </div>
    </CatalogModal>
  );
}

function ModelModal({
  item,
  initial,
  manufacturers,
  onClose,
  onSaved,
}: {
  item: Model | null;
  initial?: Partial<Model>;
  manufacturers: Manufacturer[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const source = item || initial;
  const [manufacturerId, setManufacturer] = useState(
      source?.manufacturer_id || "",
    ),
    [name, setName] = useState(source?.name || ""),
    [manufacturerName, setManufacturerName] = useState(
      source?.manufacturer_model_name || "",
    ),
    [productClass, setProductClass] = useState(source?.product_class || ""),
    [oui, setOui] = useState(source?.oui || ""),
    [description, setDescription] = useState(source?.description || "");
  const [wifi24, setWifi24] = useState(source?.supports_wifi_2_4 || false),
    [wifi5, setWifi5] = useState(source?.supports_wifi_5 || false),
    [modes, setModes] = useState<string[]>(source?.supported_auth_modes || []),
    [active, setActive] = useState(source?.active ?? true),
    [saving, setSaving] = useState(false),
    [error, setError] = useState("");
  const [newHardwareVersion, setNewHardwareVersion] = useState(
    source?.hardware_versions?.find((version) => !version.id)?.version || "",
  );
  function toggleMode(mode: string) {
    setModes((current) =>
      current.includes(mode)
        ? current.filter((v) => v !== mode)
        : [...current, mode],
    );
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    const body = {
      manufacturer_id: manufacturerId,
      name,
      manufacturer_model_name: manufacturerName || null,
      supports_wifi_2_4: wifi24,
      supports_wifi_5: wifi5,
      supported_auth_modes: modes,
      product_class: productClass || null,
      oui: oui || null,
      protocol: "TR-069",
      description: description || null,
      active,
    };
    try {
      const saved = await api<Envelope<Model>>(
        item
          ? `/api/v1/acs/router-models/${item.id}`
          : "/api/v1/acs/router-models",
        { method: item ? "PUT" : "POST", body: JSON.stringify(body) },
      );
      if (newHardwareVersion.trim())
        await api(
          `/api/v1/acs/router-models/${saved.data.id}/hardware-versions`,
          {
            method: "POST",
            body: JSON.stringify({
              version: newHardwareVersion.trim(),
              active: true,
            }),
          },
        );
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível salvar.");
    } finally {
      setSaving(false);
    }
  }
  return (
    <CatalogModal
      title={item ? "Editar modelo" : "Adicionar modelo"}
      description="Configure a identificação e as capacidades do equipamento."
      saving={saving}
      error={error}
      onClose={onClose}
      onSubmit={submit}
    >
      <div className="form-grid">
        <Field label="Fabricante">
          <select
            value={manufacturerId}
            onChange={(e) => setManufacturer(e.target.value)}
            required
          >
            <option value="">Selecione</option>
            {manufacturers
              .filter((m) => m.active || m.id === manufacturerId)
              .map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name}
                </option>
              ))}
          </select>
        </Field>
        <Field label="Nome do modelo">
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={200}
            required
          />
        </Field>
      </div>
      <div className="form-grid">
        <Field label="Nome no fabricante">
          <input
            value={manufacturerName}
            onChange={(e) => setManufacturerName(e.target.value)}
          />
        </Field>
        <Field label="Product class">
          <input
            value={productClass}
            onChange={(e) => setProductClass(e.target.value)}
          />
        </Field>
      </div>
      <div className="form-grid">
        <Field label="OUI">
          <input
            value={oui}
            onChange={(e) => setOui(e.target.value)}
            placeholder="A1B2C3"
            maxLength={8}
          />
        </Field>
        <Field label="Protocolo">
          <input value="TR-069" disabled />
        </Field>
      </div>
      <Field label="Adicionar versão de hardware">
        <input
          value={newHardwareVersion}
          onChange={(e) => setNewHardwareVersion(e.target.value)}
          placeholder="Ex.: V1, V2 ou 1.0"
          maxLength={100}
        />
      </Field>
      {!!source?.hardware_versions?.filter((version) => version.id).length && (
        <div className="hardware-list">
          <span>Versões cadastradas</span>
          <div>
            {source.hardware_versions.filter((version) => version.id).map((version) => (
              <small key={version.id}>{version.version}</small>
            ))}
          </div>
        </div>
      )}
      <Field label="Descrição">
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={3}
        />
      </Field>
      <div className="option-group">
        <span>Capacidades e autenticação</span>
        <div className="check-row">
          <label>
            <input
              type="checkbox"
              checked={wifi24}
              onChange={(e) => setWifi24(e.target.checked)}
            />{" "}
            Wi-Fi 2.4 GHz
          </label>
          <label>
            <input
              type="checkbox"
              checked={wifi5}
              onChange={(e) => setWifi5(e.target.checked)}
            />{" "}
            Wi-Fi 5 GHz
          </label>
        </div>
        <div className="check-row">
          {["WPA2_PSK", "WPA3_SAE", "WPA2_WPA3"].map((mode) => (
            <label key={mode}>
              <input
                type="checkbox"
                checked={modes.includes(mode)}
                onChange={() => toggleMode(mode)}
              />{" "}
              {mode.replaceAll("_", " ")}
            </label>
          ))}
        </div>
      </div>
      <div className="check-row">
        <label>
          <input
            type="checkbox"
            checked={active}
            onChange={(e) => setActive(e.target.checked)}
          />{" "}
          Ativo
        </label>
      </div>
    </CatalogModal>
  );
}

function PromoteRouterModal({
  router,
  models,
  onClose,
  onSaved,
}: {
  router: UnknownRouter;
  models: Model[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(
      `${router.manufacturer || "Roteador"} ${router.serial_number}`,
    ),
    [selection, setSelection] = useState(""),
    [detailedModels, setDetailedModels] = useState<Model[]>([]),
    [saving, setSaving] = useState(false),
    [error, setError] = useState("");
  useEffect(() => {
    Promise.all(
      models
        .filter((model) => model.active)
        .map((model) =>
          api<Envelope<Model>>(`/api/v1/acs/router-models/${model.id}`).then(
            (result) => result.data,
          ),
        ),
    )
      .then((items) => {
        setDetailedModels(items);
        const matchingModel = items.find(
          (model) =>
            (!model.oui || model.oui === router.oui) &&
            (!model.product_class ||
              model.product_class === router.product_class),
        );
        const matchingVersion = matchingModel?.hardware_versions?.find(
          (version) =>
            version.active &&
            version.version.localeCompare(
              router.hardware_version || "",
              undefined,
              { sensitivity: "base" },
            ) === 0,
        );
        if (matchingModel && matchingVersion)
          setSelection(`${matchingModel.id}:${matchingVersion.id}`);
      })
      .catch(() => setDetailedModels([]));
  }, [models, router.hardware_version, router.oui, router.product_class]);
  async function submit(event: FormEvent) {
    event.preventDefault();
    const [modelId, hardwareId] = selection.split(":");
    setSaving(true);
    setError("");
    try {
      await api(`/api/v1/acs/unknown-routers/${router.id}/configure`, {
        method: "POST",
        body: JSON.stringify({
          name,
          router_model_id: modelId,
          hardware_version_id: hardwareId,
        }),
      });
      onSaved();
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : "Não foi possível promover o roteador.",
      );
    } finally {
      setSaving(false);
    }
  }
  return (
    <CatalogModal
      title="Promover roteador"
      description={`Configure o equipamento ${router.serial_number} para adicioná-lo à operação.`}
      saving={saving}
      error={error}
      onClose={onClose}
      onSubmit={submit}
    >
      <Field label="Nome do roteador">
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={200}
          required
          autoFocus
        />
      </Field>
      <Field label="Modelo e versão de hardware">
        <select
          value={selection}
          onChange={(e) => setSelection(e.target.value)}
          required
        >
          <option value="">Selecione o modelo e a versão</option>
          {detailedModels.flatMap((model) =>
            (model.hardware_versions || [])
              .filter((version) => version.active)
              .map((version) => (
                <option key={version.id} value={`${model.id}:${version.id}`}>
                  {model.name} + {version.version}
                </option>
              )),
          )}
        </select>
      </Field>
      <div className="discovery-summary">
        <span>Informado pelo equipamento</span>
        <strong>
          {router.manufacturer} ·{" "}
          {router.product_class || "Product class desconhecido"}
        </strong>
        <small>
          OUI {router.oui || "—"} · Hardware {router.hardware_version || "—"}
        </small>
      </div>
    </CatalogModal>
  );
}

function CatalogModal({
  title,
  description,
  saving,
  error,
  onClose,
  onSubmit,
  children,
}: {
  title: string;
  description: string;
  saving: boolean;
  error: string;
  onClose: () => void;
  onSubmit: (e: FormEvent) => void;
  children: ReactNode;
}) {
  return (
    <div
      className="modal-backdrop"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
    >
      <section className="modal">
        <div className="modal-head">
          <div>
            <h2>{title}</h2>
            <p>{description}</p>
          </div>
          <button className="icon-button" onClick={onClose}>
            <X />
          </button>
        </div>
        <form onSubmit={onSubmit}>
          {children}
          {error && <div className="form-error">{error}</div>}
          <div className="modal-actions">
            <button type="button" className="button" onClick={onClose}>
              Cancelar
            </button>
            <button className="button primary" disabled={saving}>
              {saving ? (
                <>
                  <RefreshCw className="spin" size={15} /> Salvando
                </>
              ) : (
                "Salvar"
              )}
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}
function Firmwares({
  canManage,
  canUpload,
}: {
  canManage: boolean;
  canUpload: boolean;
}) {
  const state = useData<Envelope<Firmware[]>>("/api/v1/acs/firmwares");
  const modelsState = useData<Envelope<Model[]>>("/api/v1/acs/router-models");
  const rows = state.data?.data || [];
  const [editing, setEditing] = useState<Firmware | null | undefined>(
    undefined,
  );
  async function upload(id: string, file?: File) {
    if (!file) return;
    const body = new FormData();
    body.append("file", file);
    try {
      await api(`/api/v1/acs/firmwares/${id}/file`, { method: "POST", body });
      state.reload();
    } catch (e) {
      alert(e instanceof Error ? e.message : "Erro no upload");
    }
  }
  async function remove(item: Firmware) {
    if (!confirm(`Excluir o firmware “${item.name}” versão ${item.version}?`))
      return;
    try {
      await api(`/api/v1/acs/firmwares/${item.id}`, { method: "DELETE" });
      state.reload();
    } catch (e) {
      alert(e instanceof Error ? e.message : "Erro ao excluir");
    }
  }
  async function download(item: Firmware) {
    try {
      const file = await downloadFile(`/api/v1/acs/firmwares/${item.id}/file`);
      const url = URL.createObjectURL(file.blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = file.name;
      link.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      alert(e instanceof Error ? e.message : "Erro no download");
    }
  }
  async function makeDefault(item: Firmware) {
    if (
      !confirm(
        `Definir “${item.name}” versão ${item.version} como firmware padrão?`,
      )
    )
      return;
    try {
      await api(`/api/v1/acs/firmwares/${item.id}/default`, { method: "POST" });
      state.reload();
    } catch (e) {
      alert(e instanceof Error ? e.message : "Erro ao definir firmware padrão");
    }
  }
  async function publish(item: Firmware) {
    if (
      !confirm(
        `Publicar “${item.name}” versão ${item.version}? Depois disso, os dados e o arquivo não poderão mais ser alterados.`,
      )
    )
      return;
    try {
      await api(`/api/v1/acs/firmwares/${item.id}/publish`, { method: "POST" });
      state.reload();
    } catch (e) {
      alert(e instanceof Error ? e.message : "Erro ao publicar firmware");
    }
  }
  return (
    <Page
      title="Firmwares"
      description="Versões de software disponíveis para os equipamentos."
      action={
        <div className="page-actions">
          <button className="button" onClick={state.reload}>
            <RefreshCw size={16} /> Atualizar
          </button>
          {canManage && (
            <button className="button primary" onClick={() => setEditing(null)}>
              <Plus size={16} /> Adicionar firmware
            </button>
          )}
        </div>
      }
    >
      <State {...state} empty={!rows.length} reload={state.reload} />
      {rows.length > 0 && (
        <Table
          headers={[
            "Firmware",
            "Modelo",
            "Hardware",
            "Arquivo",
            "Publicado",
            "Status",
            "Ações",
          ]}
        >
          {rows.map((f) => (
            <tr
              className="clickable-row"
              key={f.id}
              onClick={() => setEditing(f)}
            >
              <td>
                <strong>{f.name}</strong>
                <small>
                  Versão {f.version}
                  {f.is_default ? " · Padrão" : ""}
                </small>
              </td>
              <td>{f.router_model_name}</td>
              <td>{f.hardware_version || "Todos"}</td>
              <td>
                {f.file_size != null
                  ? f.file_size >= 1073741824
                    ? `${(f.file_size / 1073741824).toFixed(1)} GB`
                    : f.file_size >= 1048576
                      ? `${(f.file_size / 1048576).toFixed(1)} MB`
                      : f.file_size >= 1024
                        ? `${(f.file_size / 1024).toFixed(1)} KB`
                        : `${f.file_size} B`
                  : "—"}
              </td>
              <td>
                <Badge value={f.active ? "PUBLISHED" : "DRAFT"} />
              </td>
              <td>
                <Badge value={f.status} />
              </td>
              <td>
                <div
                  className="row-actions"
                  onClick={(e) => e.stopPropagation()}
                >
                  {canManage && !f.active && (
                    <button
                      title="Publicar firmware"
                      onClick={() => publish(f)}
                    >
                      <CircleArrowUp />
                    </button>
                  )}
                  {canManage && f.active && !f.is_default && (
                    <button
                      title="Definir como padrão"
                      onClick={() => makeDefault(f)}
                    >
                      <Star />
                    </button>
                  )}
                  {canUpload && !f.active && (
                    <label
                      title={
                        f.status === "READY"
                          ? "Substituir arquivo"
                          : "Enviar arquivo"
                      }
                    >
                      <Upload />
                      <input
                        type="file"
                        onChange={(e) => upload(f.id, e.target.files?.[0])}
                      />
                    </label>
                  )}
                  {f.status === "READY" && (
                    <button title="Baixar arquivo" onClick={() => download(f)}>
                      <Download />
                    </button>
                  )}
                  {canManage && (
                    <button
                      className="danger"
                      title="Excluir"
                      onClick={() => remove(f)}
                    >
                      <Trash2 />
                    </button>
                  )}
                </div>
              </td>
            </tr>
          ))}
        </Table>
      )}
      {editing !== undefined && (
        <FirmwareModal
          item={editing}
          models={modelsState.data?.data || []}
          allowUpload={canUpload}
          readOnly={Boolean(editing?.active)}
          onClose={() => setEditing(undefined)}
          onSaved={() => {
            setEditing(undefined);
            state.reload();
          }}
        />
      )}
    </Page>
  );
}

function FirmwareModal({
  item,
  models,
  allowUpload,
  readOnly = false,
  onClose,
  onSaved,
}: {
  item: Firmware | null;
  models: Model[];
  allowUpload: boolean;
  readOnly?: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [modelId, setModelId] = useState(item?.router_model_id || "");
  const [hardwareId, setHardwareId] = useState(
    item?.hardware_version_id || "",
  );
  const [name, setName] = useState(item?.name || ""),
    [version, setVersion] = useState(item?.version || ""),
    [description, setDescription] = useState(item?.description || ""),
    [notes, setNotes] = useState(item?.release_notes || "");
  const [file, setFile] = useState<File | null>(null);
  const [hardware, setHardware] = useState<HardwareVersion[]>([]);
  const [saving, setSaving] = useState(false),
    [error, setError] = useState("");
  useEffect(() => {
    if (!modelId) {
      setHardware([]);
      return;
    }
    api<Envelope<Model>>(`/api/v1/acs/router-models/${modelId}`)
      .then((response) =>
        setHardware(
          (response.data.hardware_versions || []).filter(
            (version) => version.active,
          ),
        ),
      )
      .catch(() => setHardware([]));
  }, [modelId]);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    const body = {
      router_model_id: modelId,
      hardware_version_id: hardwareId || null,
      name,
      version,
      description: description || null,
      release_notes: notes || null,
    };
    try {
      const result = await api<Envelope<Firmware>>(
        item ? `/api/v1/acs/firmwares/${item.id}` : "/api/v1/acs/firmwares",
        { method: item ? "PUT" : "POST", body: JSON.stringify(body) },
      );
      if (file) {
        const form = new FormData();
        form.append("file", file);
        await api(`/api/v1/acs/firmwares/${result.data.id}/file`, {
          method: "POST",
          body: form,
        });
      }
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível salvar.");
    } finally {
      setSaving(false);
    }
  }
  return (
    <div
      className="modal-backdrop"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
    >
      <section className="modal">
        <div className="modal-head">
          <div>
            <h2>
              {readOnly
                ? "Informações do firmware"
                : item
                  ? "Editar firmware"
                  : "Adicionar firmware"}
            </h2>
            <p>
              {readOnly
                ? "Este firmware está publicado e não pode mais ser alterado."
                : item
                  ? "Edite os dados ou substitua o arquivo antes de publicar."
                  : "Cadastre uma nova versão no catálogo."}
            </p>
          </div>
          <button className="icon-button" onClick={onClose}>
            <X />
          </button>
        </div>
        <form onSubmit={submit}>
      <fieldset disabled={readOnly}>
        <div className="form-grid">
          <Field label="Modelo do roteador">
            <select
              value={modelId}
              onChange={(e) => {
                setModelId(e.target.value);
                setHardwareId("");
              }}
              required
            >
                <option value="">Selecione um modelo</option>
                {models
                  .filter((m) => m.active || m.id === modelId)
                  .map((m) => (
                    <option value={m.id} key={m.id}>
                      {m.name}
                    </option>
                  ))}
            </select>
          </Field>
          <Field label="Versão de hardware">
            <select
              value={hardwareId}
              onChange={(e) => setHardwareId(e.target.value)}
            >
              <option value="">Todos os hardwares</option>
              {hardware.map((version) => (
                <option value={version.id} key={version.id}>
                  {version.version}
                </option>
              ))}
            </select>
          </Field>
        </div>
            <div className="form-grid">
              <Field label="Nome">
                <input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  maxLength={200}
                  required
                />
              </Field>
              <Field label="Versão">
                <input
                  value={version}
                  onChange={(e) => setVersion(e.target.value)}
                  maxLength={100}
                  required
                />
              </Field>
            </div>
            <Field label="Descrição">
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={3}
              />
            </Field>
            <Field label="Notas da versão">
              <textarea
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                rows={3}
              />
            </Field>
            {readOnly && item?.file_name && (
              <Field label="Arquivo">
                <input value={item.file_name} readOnly />
              </Field>
            )}
            {!readOnly && allowUpload && (
              <Field
                label={
                  item?.file_name
                    ? "Substituir arquivo (opcional)"
                    : "Arquivo (opcional)"
                }
              >
                <label className="file-picker">
                  <Upload size={17} />
                  <span>
                    {file?.name ||
                      item?.file_name ||
                      "Selecionar arquivo de firmware"}
                  </span>
                  <input
                    type="file"
                    onChange={(e) => setFile(e.target.files?.[0] || null)}
                  />
                </label>
              </Field>
            )}
          </fieldset>
          {error && <div className="form-error">{error}</div>}
          <div className="modal-actions">
            <button type="button" className="button" onClick={onClose}>
              {readOnly ? "Fechar" : "Cancelar"}
            </button>
            {!readOnly && (
              <button className="button primary" disabled={saving}>
                {saving ? (
                  <>
                    <RefreshCw className="spin" size={15} /> Salvando
                  </>
                ) : (
                  "Salvar firmware"
                )}
              </button>
            )}
          </div>
        </form>
      </section>
    </div>
  );
}

function Toolbar({
  search,
  setSearch,
  children,
}: {
  search: string;
  setSearch: (v: string) => void;
  children?: ReactNode;
}) {
  return (
    <div className="toolbar">
      <label className="search">
        <Search size={17} />
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Buscar por nome ou serial..."
        />
      </label>
      {children}
    </div>
  );
}
function Table({
  headers,
  children,
}: {
  headers: string[];
  children: ReactNode;
}) {
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            {headers.map((h, i) => (
              <th key={i}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}
function Field({
  label: text,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <label className="field">
      <span>{text}</span>
      {children}
    </label>
  );
}

export default App;
