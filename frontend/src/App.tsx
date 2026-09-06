import { FormEvent, ReactNode, useCallback, useEffect, useState } from "react";
import {
  Activity,
  ArrowRight,
  ArrowLeft,
  Bell,
  BarChart3,
  Boxes,
  ChevronRight,
  CircleArrowUp,
  CircleGauge,
  Command,
  Cpu,
  Download,
  Eye,
  EyeOff,
  Filter,
  LogOut,
  LockKeyhole,
  Mail,
  Menu,
  PackageOpen,
  Plus,
  RefreshCw,
  RotateCcw,
  Router,
  Search,
  Settings,
  ShieldCheck,
  Star,
  ShieldBan,
  Trash2,
  Upload,
  Pencil,
  TriangleAlert,
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
  useParams,
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
type RouterDetailData = RouterItem & {
  oui: string;
  product_class: string;
  hardware_version?: string;
  catalog_hardware_version?: string;
  reported_status: string;
  first_seen_at?: string;
  last_inform_at?: string;
  created_at: string;
  updated_at: string;
  inform_count: number;
  firmware_updated_at?: string;
  collected_parameters: Record<string, string>;
  last_sync_at?: string;
  band_steering_enabled?: boolean;
  configuration_pending: boolean;
  router_model_id?: string;
  hardware_version_id?: string;
  network?: { dhcp_enabled: boolean; dhcp_start?: string; dhcp_end?: string; dhcp_gateway?: string; dhcp_subnet_mask?: string; dhcp_lease_seconds?: number; dns_primary?: string; dns_secondary?: string };
  wifi: { band: string; ssid: string; password_configured: boolean; security_mode: string; channel?: number; channel_mode: string; bandwidth_mhz?: number; enabled: boolean }[];
  sessions: { id: string; ip_address?: string; started_at: string; status: string }[];
  commands: { id: string; type: string; status: string; attempts_count: number; created_at: string; completed_at?: string }[];
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
type TR069Command = {
  id?: string;
  parameter_path: string;
  value_type: string;
  active: boolean;
};
type TR069DataModel = {
  id: string;
  name: string;
  active: boolean;
  commands: TR069Command[];
};
type Model = {
  id: string;
  manufacturer_id: string;
  name: string;
  manufacturer_name: string;
  tr069_data_model_id?: string;
  tr069_data_model_name?: string;
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
  const tenant = import.meta.env.VITE_TENANT_SLUG || "netcontrol";
  const [email, setEmail] = useState(""),
    [password, setPassword] = useState(""),
    [showPassword, setShowPassword] = useState(false),
    [remember, setRemember] = useState(true);
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
      <section className="login-shell">
        <aside className="login-showcase">
          <div className="login-brand showcase-brand"><span className="brand-mark"><Wifi size={22} /></span><span>net<span>control</span></span></div>
          <div className="showcase-copy">
            <p className="showcase-eyebrow">Rede mais simples,<br />negócios mais fortes</p>
            <h1>Controle sua rede<br />com mais eficiência</h1>
            <p>Monitore, configure e gerencie seus equipamentos de rede em um só lugar, de forma simples e segura.</p>
          </div>
          <div className="showcase-features">
            <div><span><BarChart3 /></span><p><strong>Visibilidade total</strong>Acompanhe o status da sua rede em tempo real.</p></div>
            <div><span><Settings /></span><p><strong>Gestão centralizada</strong>Configure e gerencie todos os equipamentos em um só lugar.</p></div>
            <div><span><ShieldCheck /></span><p><strong>Mais segurança</strong>Mantenha sua infraestrutura sempre protegida.</p></div>
          </div>
          <div className="network-illustration"><div className="signal-card"><Wifi /></div><div className="bars-card"><BarChart3 /></div><div className="router-box"><Wifi /><i /><i /><i /></div><div className="router-box lower"><span /><span /><span /><i /><i /></div></div>
          <p className="showcase-tagline">Tecnologia que mantém<br />o seu mundo conectado.</p>
        </aside>
        <section className="login-card">
          <div className="login-brand"><span className="brand-mark"><Wifi size={20} /></span><span>net<span>control</span></span></div>
          <div className="login-copy"><p className="eyebrow">Bem-vindo de volta</p><h1>Acesse sua conta</h1><p>Gerencie seus equipamentos de rede em um só lugar.</p></div>
          <form onSubmit={submit}>
            <Field label="E-mail"><div className="login-input"><Mail size={18} /><input value={email} onChange={(e) => setEmail(e.target.value)} type="email" placeholder="admin@netcontrol.local" autoComplete="email" required autoFocus /></div></Field>
            <Field label="Senha"><div className="login-input"><LockKeyhole size={18} /><input value={password} onChange={(e) => setPassword(e.target.value)} type={showPassword ? "text" : "password"} placeholder="Sua senha" autoComplete="current-password" required /><button type="button" onClick={() => setShowPassword(value => !value)} aria-label={showPassword ? "Ocultar senha" : "Mostrar senha"}>{showPassword ? <EyeOff size={18} /> : <Eye size={18} />}</button></div></Field>
            <div className="login-options"><label><input type="checkbox" checked={remember} onChange={event => setRemember(event.target.checked)} /> <span>Lembrar de mim</span></label></div>
            {error && <div className="form-error">{error}</div>}
            <button className="button primary login-button" disabled={loading}>{loading ? <><RefreshCw className="spin" size={16} /> Entrando...</> : <>Entrar <ArrowRight size={18} /></>}</button>
          </form>
          <div className="login-divider"><span>ou</span></div>
          <p className="login-foot"><LockKeyhole size={15} /> Acesso restrito a usuários autorizados</p>
        </section>
      </section>
      <footer className="login-page-footer"><span>© {new Date().getFullYear()} NetControl. Todos os direitos reservados.</span><span>Suporte &nbsp;&nbsp;&nbsp; Documentação &nbsp;&nbsp;&nbsp; Política de Privacidade</span></footer>
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
          <div className="mobile-brand">
            <span>NetControl</span>
            <small>Módulo ACS</small>
          </div>
          <div className="global-search">
            <Search size={17} />
            <span>Buscar roteadores, comandos...</span>
            <kbd>⌘ K</kbd>
          </div>
          <button className="header-notification" title="Notificações"><Bell size={19} /><i /></button>
          <div className="header-avatar">{user.name.slice(0, 2).toUpperCase()}</div>
        </header>
        <Routes>
          <Route path="/" element={<Navigate to={first} replace />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/routers" element={<Routers canDelete={user.permissions.includes("acs.router.delete")} canBlock={user.permissions.includes("acs.router.command.block")} />} />
          <Route path="/routers/:id" element={<RouterDetail />} />
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
  titleExtra,
  children,
}: {
  title: string;
  description: string;
  action?: ReactNode;
  titleExtra?: ReactNode;
  children: ReactNode;
}) {
  return (
    <main className="page">
      <div className="page-title">
        <div>
          <div className="page-heading"><h1>{title}</h1>{titleExtra}</div>
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

function Routers({ canDelete, canBlock }: { canDelete: boolean; canBlock: boolean }) {
  const navigate = useNavigate();
  const [search, setSearch] = useState(""),
    [status, setStatus] = useState("");
  const query = new URLSearchParams({ limit: "100" });
  if (search) query.set("search", search);
  if (status) query.set("connectivity_status", status);
  const state = useData<ListEnvelope<RouterItem>>(
    `/api/v1/acs/routers?${query}`,
  );
  const rows = state.data?.data || [];
  async function remove(router: RouterItem) { if (!confirm(`Excluir o roteador “${router.name}” e todo o seu histórico?`)) return; try { await api(`/api/v1/acs/routers/${router.id}`, { method: "DELETE" }); state.reload() } catch (e) { alert(e instanceof Error ? e.message : "Erro ao excluir") } }
  async function block(router: RouterItem) { if (!confirm(`Bloquear o roteador “${router.name}”?`)) return; try { await api(`/api/v1/acs/routers/${router.id}/block`, { method: "POST" }); state.reload() } catch (e) { alert(e instanceof Error ? e.message : "Erro ao bloquear") } }
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
            "Ações",
          ]}
        >
          {rows.map((r) => (
            <tr className="clickable-row" key={r.id} onClick={() => navigate(`/routers/${r.id}`)}>
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
              <td><div className="row-actions" onClick={(e) => e.stopPropagation()}>{canBlock && r.desired_status !== "BLOCKED" && <button title="Bloquear" onClick={() => block(r)}><ShieldBan /></button>}{canDelete && <button className="danger" title="Excluir" onClick={() => remove(r)}><Trash2 /></button>}</div></td>
            </tr>
          ))}
        </Table>
      )}
    </Page>
  );
}
type RouterTab = "general" | "wifi" | "wifi24" | "wifi5" | "wan" | "lan";
function RouterDetail() {
  const { id } = useParams(), navigate = useNavigate();
  const state = useData<Envelope<RouterDetailData>>(`/api/v1/acs/routers/${id}`), router = state.data?.data;
  const [tab, setTab] = useState<RouterTab>("general"), [editing, setEditing] = useState<Exclude<RouterTab, "general"> | undefined>();
  const collected = router?.collected_parameters || {};
  const parameter = (...names: string[]) => names.map(name => collected[name]).find(value => value !== undefined && value !== "");
  const matchingParameter = (pattern: RegExp) => Object.entries(collected).find(([name, value]) => pattern.test(name) && value !== "")?.[1];
  const booleanParameter = (...names: string[]) => { const value = parameter(...names); return value === undefined ? undefined : ["1", "true", "enabled", "up"].includes(value.toLowerCase()) ? "Ativo" : "Desativado"; };
  const reportedBandSteering = parameter("Device.WiFi.X_TP_BandSteering.Enable");
  const bandSteeringEnabled = reportedBandSteering !== undefined
    ? ["1", "true", "enabled", "enable", "on"].includes(reportedBandSteering.toLowerCase())
    : (router?.band_steering_enabled ?? false);
  useEffect(() => {
    if (bandSteeringEnabled && (tab === "wifi24" || tab === "wifi5")) setTab("wifi");
    if (!bandSteeringEnabled && tab === "wifi") setTab("wifi24");
  }, [bandSteeringEnabled, tab]);
  const wanDNS = (parameter("Device.DHCPv4.Client.1.DNSServers", "Device.DNS.Relay.Forwarding.1.DNSServer") || "").split(",").map(value => value.trim()).filter(value => value && value !== "0.0.0.0"), lanDNS = (parameter("Device.DHCPv4.Server.Pool.1.DNSServers") || "").split(",").map(value => value.trim()).filter(value => value && value !== "0.0.0.0");
  async function block() { if (!confirm("Bloquear este roteador?")) return; try { await api(`/api/v1/acs/routers/${id}/block`, { method: "POST" }); state.reload() } catch (e) { alert(e instanceof Error ? e.message : "Erro ao bloquear") } }
  async function changeBandSteering(enabled: boolean) { const action = enabled ? "Habilitar" : "Desabilitar"; if (!confirm(`${action} o Band Steering? A alteração será aplicada quando você executar Ativar.`)) return; try { await api(`/api/v1/acs/routers/${id}/configuration`, { method: "PUT", body: JSON.stringify({ wifi: [], network: null, band_steering_enabled: enabled }) }); await state.reload(); setTab(enabled ? "wifi" : "wifi24"); } catch (e) { alert(e instanceof Error ? e.message : `Erro ao ${action.toLowerCase()} Band Steering`); } }
  async function revertConfiguration() { if (!confirm("Descartar todas as alterações ainda não aplicadas?")) return; try { await api(`/api/v1/acs/routers/${id}/configuration/revert`, { method: "POST" }); await state.reload(); } catch (e) { alert(e instanceof Error ? e.message : "Erro ao reverter alterações"); } }
  async function runCommand(type: string) { if (!type) return; if (type === "BLOCK") return block(); if ((type === "REBOOT" || type === "FACTORY_RESET") && !confirm(type === "REBOOT" ? "Reiniciar este roteador?" : "Redefinir este roteador para as configurações de fábrica?")) return; try { const body: Record<string, string> = { type }; if (type === "UPDATE_FIRMWARE") { const result = await api<Envelope<Firmware[]>>("/api/v1/acs/firmwares"); const options = result.data.filter(item => item.status === "READY" && item.active && item.router_model_id === router?.router_model_id && (!item.hardware_version_id || item.hardware_version_id === router?.hardware_version_id)); const firmware = options.find(item => item.is_default) || options[0]; if (!firmware) throw new Error("Nenhum firmware compatível e publicado foi encontrado."); if (!confirm(`Atualizar para o firmware ${firmware.name} ${firmware.version}?`)) return; body.firmware_id = firmware.id; } await api(`/api/v1/acs/routers/${id}/commands`, { method: "POST", body: JSON.stringify(body) }); alert("Comando adicionado à fila."); state.reload(); } catch (e) { alert(e instanceof Error ? e.message : "Erro ao gerar comando"); } }
  const tabs: { key: RouterTab; title: string }[] = [{ key: "general", title: "Geral" }, ...(bandSteeringEnabled ? [{ key: "wifi" as RouterTab, title: "Rede 2.4/5.8 GHz" }] : [{ key: "wifi24" as RouterTab, title: "Rede 2.4 GHz" }, { key: "wifi5" as RouterTab, title: "Rede 5.8 GHz" }]), { key: "wan", title: "WAN" }, { key: "lan", title: "LAN" }];
  return <Page title={router?.name || "Roteador"} titleExtra={router && <span className="operational-badge"><i /> Sistema operacional</span>} description="Informações, redes e operação do equipamento." action={<div className="page-actions"><button className="button" onClick={() => navigate("/routers")}><ArrowLeft size={16} /> Voltar</button><button className="button" onClick={state.reload}><RefreshCw size={16} /> Atualizar</button>{router?.configuration_pending && <button className="button danger-button" onClick={revertConfiguration}><RotateCcw size={16} /> Reverter alterações</button>}<select className="action-select" defaultValue="" onChange={event => { void runCommand(event.target.value); event.target.value = ""; }}><option value="" disabled>Ações</option><option value="ACTIVATE">Ativar</option><option value="BLOCK">Bloquear</option><option value="REBOOT">Reiniciar</option><option value="FACTORY_RESET">Redefinir de fábrica</option><option value="UPDATE_FIRMWARE">Atualizar firmware</option></select><a className="button primary" href="#router-commands"><Command size={16} /> Listar comandos</a></div>}>
    <State {...state} reload={state.reload} />{router && <>{router.configuration_pending && <div className="pending-alert"><span className="pending-alert-icon"><TriangleAlert size={25} /></span><div><strong>Este roteador possui alterações que ainda não foram aplicadas.</strong><p>As configurações foram alteradas, mas ainda não estão ativas no equipamento.</p></div><button className="button" onClick={revertConfiguration}>Reverter alterações</button></div>}
      <section className="detail-section router-tab-card"><div className="tabs">{tabs.map(item => <button key={item.key} className={tab === item.key ? "active" : ""} onClick={() => setTab(item.key)}>{item.title}</button>)}</div>{tab !== "general" && <div className="router-section-heading"><div className="section-title-copy"><span className="section-icon">{(tab === "wifi" || tab === "wifi24" || tab === "wifi5") ? <Wifi /> : <Router />}</span><div><h2>{tab === "wifi" ? "Redes Wi-Fi" : tab === "wifi24" ? "Rede 2.4 GHz" : tab === "wifi5" ? "Rede 5.8 GHz" : tab === "wan" ? "Configurações WAN" : "Configurações LAN"}</h2><p>{tab.startsWith("wifi") ? "Configurações das redes sem fio 2.4 GHz e 5 GHz." : "Parâmetros de rede configurados para este equipamento."}</p></div></div><div className="tab-actions">{tab === "wifi" ? <button className="button danger-button" onClick={() => void changeBandSteering(false)}>Desabilitar Band Steering</button> : (tab === "wifi24" || tab === "wifi5") && <button className="button" onClick={() => void changeBandSteering(true)}>Habilitar Band Steering</button>}<button className="button primary-outline" onClick={() => setEditing(tab)}><Pencil size={16} /> Editar</button></div></div>}
      {tab === "general" && <div className="info-grid">
        <Info label="Endereço IPv4" value={matchingParameter(/^Device\.IP\.Interface\.\d+\.IPv4Address\.\d+\.IPAddress$/) || router.last_ip_address} mono /><Info label="Endereço IPv6" value={matchingParameter(/^Device\.IP\.Interface\.\d+\.IPv6Address\.\d+\.IPAddress$/)} mono /><Info label="Status" value={label[router.connectivity_status] || router.connectivity_status} /><Info label="OUI" value={parameter("Device.DeviceInfo.ManufacturerOUI") || router.oui} mono /><Info label="Nome" value={router.name} /><Info label="UPNP" value={booleanParameter("Device.UPnP.Device.Enable", "Device.UPnP.Enable")} /><Info label="Modelo" value={parameter("Device.DeviceInfo.ModelName") || [router.router_model_name, router.catalog_hardware_version].filter(Boolean).join(" + ")} /><Info label="Número de série" value={parameter("Device.DeviceInfo.SerialNumber") || router.serial_number} mono /><Info label="MAC" value={parameter("Device.WiFi.SSID.1.MACAddress", "Device.Ethernet.Interface.1.MACAddress") || matchingParameter(/\.MACAddress$/)} mono /><Info label="Classe do produto" value={router.product_class} /><Info label="Firmware" value={parameter("Device.DeviceInfo.AdditionalSoftwareVersion", "Device.DeviceInfo.SoftwareVersion")} /><Info label="Versão do software" value={parameter("Device.DeviceInfo.SoftwareVersion") || router.software_version} /><Info label="Versão do hardware" value={parameter("Device.DeviceInfo.HardwareVersion") || router.catalog_hardware_version || router.hardware_version} /><Info label="Informes recebidos" value={String(router.inform_count)} /><Info label="Último informe" value={formatDate(router.last_inform_at)} /><Info label="Última sincronização" value={formatDate(router.last_sync_at)} /><Info label="Criado em" value={formatDate(router.created_at)} /><Info label="Atualizado em" value={formatDate(router.updated_at)} /><Info label="Firmware atualizado em" value={formatDate(router.firmware_updated_at)} />
      </div>}{tab === "wifi" && <div className="steered-wifi"><WiFiDetails band="2.4GHZ" title="Rede 2.4/5.8 GHz" router={router} parameter={parameter} booleanParameter={booleanParameter} /></div>}{(tab === "wifi24" || tab === "wifi5") && <WiFiDetails band={tab === "wifi24" ? "2.4GHZ" : "5GHZ"} router={router} parameter={parameter} booleanParameter={booleanParameter} />}{tab === "wan" && <div className="info-grid"><Info label="Status" value={parameter("Device.DHCPv4.Client.1.Status", "Device.DHCPv4.Client.1.DHCPStatus")} /><Info label="Endereço IPv4" value={parameter("Device.DHCPv4.Client.1.IPAddress", "Device.IP.Interface.2.IPv4Address.1.IPAddress")} /><Info label="Gateway" value={parameter("Device.DHCPv4.Client.1.IPRouters", "Device.Routing.Router.1.IPv4Forwarding.1.GatewayIPAddress")} /><Info label="Máscara" value={parameter("Device.DHCPv4.Client.1.SubnetMask", "Device.IP.Interface.2.IPv4Address.1.SubnetMask")} /><Info label="DNS primário" value={router.network?.dns_primary || wanDNS[0]} /><Info label="DNS secundário" value={router.network?.dns_secondary || wanDNS[1]} /></div>}{tab === "lan" && <div className="info-grid"><Info label="DHCP" value={router.network ? (router.network.dhcp_enabled ? "Ativo" : "Desativado") : booleanParameter("Device.DHCPv4.Server.Pool.1.Enable", "Device.DHCPv4.Server.Enable")} /><Info label="Início do range" value={router.network?.dhcp_start || parameter("Device.DHCPv4.Server.Pool.1.MinAddress")} /><Info label="Fim do range" value={router.network?.dhcp_end || parameter("Device.DHCPv4.Server.Pool.1.MaxAddress")} /><Info label="Gateway" value={router.network?.dhcp_gateway || parameter("Device.IP.Interface.1.IPv4Address.1.IPAddress")} /><Info label="Máscara" value={router.network?.dhcp_subnet_mask || parameter("Device.DHCPv4.Server.Pool.1.SubnetMask", "Device.IP.Interface.1.IPv4Address.1.SubnetMask")} /><Info label="Tempo de concessão" value={router.network?.dhcp_lease_seconds ? `${router.network.dhcp_lease_seconds}s` : parameter("Device.DHCPv4.Server.Pool.1.LeaseTime")} /><Info label="DNS primário dos clientes" value={router.network?.dns_primary || lanDNS[0]} /><Info label="DNS secundário dos clientes" value={router.network?.dns_secondary || lanDNS[1]} /></div>}</section>
      <section className="detail-section commands-card" id="router-commands"><div className="commands-header"><div className="section-title-copy"><span className="section-icon"><Command /></span><div><h2>Comandos recentes</h2><p>Histórico dos últimos comandos enviados para este roteador.</p></div></div><div className="commands-tools"><label><Search size={16} /><input placeholder="Buscar comandos..." /></label><button className="button"><Filter size={16} /> Filtros</button></div></div>{router.commands.length ? <Table headers={["Comando", "Status", "Tentativas", "Criado em", "Concluído em"]}>{router.commands.map(item => <tr key={item.id}><td>{label[item.type] || item.type}</td><td><Badge value={item.status} /></td><td>{item.attempts_count}</td><td>{formatDate(item.created_at)}</td><td>{formatDate(item.completed_at)}</td></tr>)}</Table> : <EmptyInline />}<div className="table-summary">Mostrando {router.commands.length} comando{router.commands.length === 1 ? "" : "s"}</div></section>
      <section className="detail-section"><h2>Parâmetros coletados</h2>{Object.keys(collected).length ? <Table headers={["Parâmetro", "Valor"]}>{Object.entries(collected).map(([name, value]) => <tr key={name}><td className="mono">{name}</td><td>{/(password|passphrase|presharedkey|secret)/i.test(name) && value ? "••••••••" : value || "—"}</td></tr>)}</Table> : <EmptyInline />}</section>
      {editing && <RouterConfigurationModal kind={editing} router={router} parameter={parameter} onClose={() => setEditing(undefined)} onSaved={() => { setEditing(undefined); state.reload(); }} />}
    </>}</Page>
}
function WiFiDetails({ band, title, router, parameter, booleanParameter }: { band: string; title?: string; router: RouterDetailData; parameter: (...names: string[]) => string | undefined; booleanParameter: (...names: string[]) => string | undefined }) { const wifi = router.wifi.find(item => item.band === band), index = band === "2.4GHZ" ? 1 : 2, radio = `Device.WiFi.Radio.${index}`, ssid = `Device.WiFi.SSID.${index}`, accessPoint = `Device.WiFi.AccessPoint.${index}`, password = parameter(`${accessPoint}.Security.KeyPassphrase`, `${accessPoint}.Security.PreSharedKey`), enabled = booleanParameter(`${radio}.Enable`, `${accessPoint}.Enable`) || (wifi ? (wifi.enabled ? "Ativo" : "Desativado") : undefined); return <div className="wifi-band-card"><div className="wifi-band-title"><Wifi size={24} /><h3>{title || `Rede ${band === "2.4GHZ" ? "2.4" : "5"} GHz`}</h3>{enabled && <span className={enabled === "Ativo" ? "state-pill active" : "state-pill"}>{enabled}</span>}</div><div className="info-grid"><Info label="SSID" value={parameter(`${ssid}.SSID`) || wifi?.ssid} /><Info label="Modo" value={parameter(`${radio}.OperatingStandards`) || wifi?.channel_mode} /><Info label="Segurança" value={parameter(`${accessPoint}.Security.ModeEnabled`) || wifi?.security_mode} /><Info label="Senha" value={password !== undefined ? (password ? "Configurada" : "Não configurada") : wifi?.password_configured ? "Configurada" : undefined} /><Info label="Canal" value={parameter(`${radio}.Channel`) || wifi?.channel?.toString()} /><Info label="Largura de banda" value={parameter(`${radio}.OperatingChannelBandwidth`) || (wifi?.bandwidth_mhz ? `${wifi.bandwidth_mhz} MHz` : undefined)} /><Info label="Potência" value={parameter(`${radio}.TransmitPower`)} /><Info label="Ativo" value={enabled} /><Info label="Visível" value={booleanParameter(`${accessPoint}.SSIDAdvertisementEnabled`)} /><Info label="Isolar clientes" value={booleanParameter(`${accessPoint}.IsolationEnable`)} /></div></div> }
function RouterConfigurationModal({ kind, router, parameter, onClose, onSaved }: { kind: Exclude<RouterTab, "general">; router: RouterDetailData; parameter: (...names: string[]) => string | undefined; onClose: () => void; onSaved: () => void }) {
  const band = kind === "wifi5" ? "5GHZ" : "2.4GHZ", index = kind === "wifi5" ? 2 : 1, current = router.wifi.find(item => item.band === band);
  const [ssid, setSSID] = useState(current?.ssid || parameter(`Device.WiFi.SSID.${index}.SSID`) || ""), [password, setPassword] = useState(""), [security, setSecurity] = useState(current?.security_mode || "WPA2_PSK"), [channelMode, setChannelMode] = useState(current?.channel_mode || "AUTO"), [channel, setChannel] = useState(current?.channel?.toString() || ""), [bandwidth, setBandwidth] = useState(current?.bandwidth_mhz?.toString() || ""), [enabled, setEnabled] = useState(current?.enabled ?? true);
  const network = router.network || { dhcp_enabled: false }, reportedDNS = (parameter(kind === "wan" ? "Device.DHCPv4.Client.1.DNSServers" : "Device.DHCPv4.Server.Pool.1.DNSServers") || "").split(",").map(value => value.trim()).filter(value => value && value !== "0.0.0.0"), [dhcpEnabled, setDHCPEnabled] = useState(router.network ? network.dhcp_enabled : parameter("Device.DHCPv4.Server.Pool.1.Enable", "Device.DHCPv4.Server.Enable") === "1"), [dhcpStart, setDHCPStart] = useState(network.dhcp_start || parameter("Device.DHCPv4.Server.Pool.1.MinAddress") || ""), [dhcpEnd, setDHCPEnd] = useState(network.dhcp_end || parameter("Device.DHCPv4.Server.Pool.1.MaxAddress") || ""), [gateway, setGateway] = useState(network.dhcp_gateway || parameter("Device.IP.Interface.1.IPv4Address.1.IPAddress") || ""), [mask, setMask] = useState(network.dhcp_subnet_mask || parameter("Device.DHCPv4.Server.Pool.1.SubnetMask") || ""), [lease, setLease] = useState(network.dhcp_lease_seconds?.toString() || parameter("Device.DHCPv4.Server.Pool.1.LeaseTime") || "86400"), [dnsPrimary, setDNSPrimary] = useState(network.dns_primary || reportedDNS[0] || ""), [dnsSecondary, setDNSSecondary] = useState(network.dns_secondary || reportedDNS[1] || ""), [saving, setSaving] = useState(false), [error, setError] = useState("");
  async function submit(event: FormEvent) { event.preventDefault(); setSaving(true); setError(""); try { const wifiBands = kind === "wifi" ? ["2.4GHZ", "5GHZ"] : [band], body = kind === "wifi" || kind === "wifi24" || kind === "wifi5" ? { wifi: wifiBands.map(wifiBand => ({ band: wifiBand, ssid, password, security_mode: security, channel_mode: channelMode, channel: channelMode === "MANUAL" ? Number(channel) : null, bandwidth_mhz: bandwidth ? Number(bandwidth) : null, enabled })), network: null, band_steering_enabled: kind === "wifi" ? true : router.band_steering_enabled ?? null } : { wifi: [], network: { dhcp_enabled: dhcpEnabled, dhcp_start: dhcpStart || null, dhcp_end: dhcpEnd || null, dhcp_gateway: gateway || null, dhcp_subnet_mask: mask || null, dhcp_lease_seconds: dhcpEnabled ? Number(lease) : null, dns_primary: dnsPrimary || null, dns_secondary: dnsSecondary || null }, band_steering_enabled: null }; await api(`/api/v1/acs/routers/${router.id}/configuration`, { method: "PUT", body: JSON.stringify(body) }); onSaved(); } catch (requestError) { setError(requestError instanceof Error ? requestError.message : "Não foi possível salvar."); } finally { setSaving(false); } }
  const wifi = kind === "wifi" || kind === "wifi24" || kind === "wifi5";
  return <CatalogModal title={`Editar ${kind === "wifi24" ? "rede 2.4 GHz" : kind === "wifi5" ? "rede 5.8 GHz" : kind.toUpperCase()}`} description="A alteração será aplicada ao roteador quando você executar a ação Ativar." saving={saving} error={error} onClose={onClose} onSubmit={submit}>{wifi ? <><Field label="SSID"><input value={ssid} onChange={event => setSSID(event.target.value)} required maxLength={32} /></Field><Field label="Nova senha"><input type="text" value={password} onChange={event => setPassword(event.target.value)} required minLength={8} maxLength={63} /></Field><div className="form-grid"><Field label="Segurança"><select value={security} onChange={event => setSecurity(event.target.value)}><option value="WPA2_PSK">WPA2</option><option value="WPA3_SAE">WPA3</option><option value="WPA2_WPA3">WPA2/WPA3</option></select></Field><Field label="Canal"><select value={channelMode} onChange={event => setChannelMode(event.target.value)}><option value="AUTO">Automático</option><option value="MANUAL">Manual</option></select></Field></div>{channelMode === "MANUAL" && <Field label="Número do canal"><input type="number" min={1} max={196} value={channel} onChange={event => setChannel(event.target.value)} required /></Field>}<Field label="Largura de banda (MHz)"><input type="number" value={bandwidth} onChange={event => setBandwidth(event.target.value)} /></Field><div className="check-row"><label><input type="checkbox" checked={enabled} onChange={event => setEnabled(event.target.checked)} /> Ativo</label></div></> : kind === "wan" ? <div className="form-grid"><Field label="DNS primário"><input value={dnsPrimary} onChange={event => setDNSPrimary(event.target.value)} placeholder="1.1.1.1" /></Field><Field label="DNS secundário"><input value={dnsSecondary} onChange={event => setDNSSecondary(event.target.value)} placeholder="8.8.8.8" /></Field></div> : <><div className="check-row"><label><input type="checkbox" checked={dhcpEnabled} onChange={event => setDHCPEnabled(event.target.checked)} /> Servidor DHCP ativo</label></div><div className="form-grid"><Field label="Início do range"><input value={dhcpStart} onChange={event => setDHCPStart(event.target.value)} /></Field><Field label="Fim do range"><input value={dhcpEnd} onChange={event => setDHCPEnd(event.target.value)} /></Field><Field label="Gateway"><input value={gateway} onChange={event => setGateway(event.target.value)} /></Field><Field label="Máscara"><input value={mask} onChange={event => setMask(event.target.value)} /></Field><Field label="Concessão (segundos)"><input type="number" value={lease} onChange={event => setLease(event.target.value)} /></Field><Field label="DNS primário dos clientes"><input value={dnsPrimary} onChange={event => setDNSPrimary(event.target.value)} placeholder="1.1.1.1" /></Field><Field label="DNS secundário dos clientes"><input value={dnsSecondary} onChange={event => setDNSSecondary(event.target.value)} placeholder="8.8.8.8" /></Field></div></>}</CatalogModal>;
}
function Info({ label: title, value, mono = false }: { label: string; value?: string | null; mono?: boolean }) { return <div className="info-item"><span>{title}</span><strong className={mono ? "mono" : ""}>{value || "Não informado"}</strong></div> }
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
    ),
    dataModelsState = useData<Envelope<TR069DataModel[]>>(
      "/api/v1/acs/tr069-data-models",
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
            dataModelsState.reload();
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
          dataModels={dataModelsState.data?.data || []}
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
      action={
        <button className="button" onClick={state.reload}>
          <RefreshCw size={16} /> Atualizar
        </button>
      }
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
              <td>
                {label[r.type] || r.type}
                {r.error_message && <small>{r.error_message}</small>}
              </td>
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
  const [tab, setTab] = useState<"models" | "manufacturers" | "data-models">(
      "models",
    ),
    [modelModal, setModelModal] = useState<Model | null | undefined>(undefined),
    [manufacturerModal, setManufacturerModal] = useState<
      Manufacturer | null | undefined
    >(undefined),
    [dataModelModal, setDataModelModal] = useState<
      TR069DataModel | null | undefined
    >(undefined);
  const modelsState = useData<Envelope<Model[]>>("/api/v1/acs/router-models"),
    manufacturersState = useData<Envelope<Manufacturer[]>>(
      "/api/v1/acs/manufacturers",
    ),
    dataModelsState = useData<Envelope<TR069DataModel[]>>(
      "/api/v1/acs/tr069-data-models",
    );
  const state =
      tab === "models"
        ? modelsState
        : tab === "manufacturers"
          ? manufacturersState
          : dataModelsState,
    rows = state.data?.data || [];
  function reload() {
    if (tab === "models") modelsState.reload();
    else if (tab === "manufacturers") manufacturersState.reload();
    else dataModelsState.reload();
  }
  async function remove(
    kind: "model" | "manufacturer" | "data-model",
    id: string,
    name: string,
  ) {
    if (!confirm(`Excluir “${name}”?`)) return;
    try {
      await api(
        `/api/v1/acs/${kind === "model" ? "router-models" : kind === "manufacturer" ? "manufacturers" : "tr069-data-models"}/${id}`,
        { method: "DELETE" },
      );
      if (kind === "model") modelsState.reload();
      else if (kind === "manufacturer") manufacturersState.reload();
      else dataModelsState.reload();
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
          onClick={() => {
            if (tab === "models") setModelModal(null);
            else if (tab === "manufacturers") setManufacturerModal(null);
            else setDataModelModal(null);
          }}
        >
          <Plus size={16} /> Adicionar{" "}
          {tab === "models"
            ? "modelo"
            : tab === "manufacturers"
              ? "fabricante"
              : "modelo de dados"}
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
        <button
          className={tab === "data-models" ? "active" : ""}
          onClick={() => setTab("data-models")}
        >
          Modelos de dados TR-069
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
        ) : tab === "manufacturers" ? (
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
        ) : (
          <Table headers={["Nome", "Comandos", "Status", "Ações"]}>
            {(rows as TR069DataModel[]).map((dataModel) => (
              <tr
                className={canManage ? "clickable-row" : ""}
                key={dataModel.id}
                onClick={() => canManage && setDataModelModal(dataModel)}
              >
                <td>
                  <strong>{dataModel.name}</strong>
                </td>
                <td>{dataModel.commands.length}</td>
                <td>
                  <Badge value={dataModel.active ? "ACTIVE" : "INACTIVE"} />
                </td>
                <td>
                  {canManage && (
                    <div
                      className="row-actions"
                      onClick={(event) => event.stopPropagation()}
                    >
                      <button
                        className="danger"
                        title="Excluir"
                        onClick={() =>
                          remove("data-model", dataModel.id, dataModel.name)
                        }
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
          dataModels={dataModelsState.data?.data || []}
          onClose={() => setModelModal(undefined)}
          onSaved={() => {
            setModelModal(undefined);
            modelsState.reload();
          }}
        />
      )}
      {dataModelModal !== undefined && (
        <TR069DataModelModal
          item={dataModelModal}
          onClose={() => setDataModelModal(undefined)}
          onSaved={() => {
            setDataModelModal(undefined);
            dataModelsState.reload();
          }}
        />
      )}
    </Page>
  );
}

const tr069ValueTypes = [
  "string",
  "boolean",
  "int",
  "unsignedInt",
  "dateTime",
  "base64",
  "hexBinary",
];

function parseCSVRow(row: string): string[] {
  const fields: string[] = [];
  let field = "";
  let quoted = false;
  for (let index = 0; index < row.length; index++) {
    const character = row[index];
    if (character === '"') {
      if (quoted && row[index + 1] === '"') {
        field += '"';
        index++;
      } else {
        quoted = !quoted;
      }
    } else if (character === "," && !quoted) {
      fields.push(field.trim());
      field = "";
    } else {
      field += character;
    }
  }
  if (quoted) throw new Error("Há aspas não fechadas no arquivo CSV.");
  fields.push(field.trim());
  return fields;
}

function TR069DataModelModal({
  item,
  onClose,
  onSaved,
}: {
  item: TR069DataModel | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(item?.name || ""),
    [active, setActive] = useState(item?.active ?? true),
    [commands, setCommands] = useState<TR069Command[]>(
      item?.commands || [
        { parameter_path: "", value_type: "string", active: true },
      ],
    ),
    [saving, setSaving] = useState(false),
    [error, setError] = useState(""),
    [importMessage, setImportMessage] = useState("");
  function updateCommand(
    index: number,
    field: "parameter_path" | "value_type",
    value: string,
  ) {
    setCommands((current) =>
      current.map((command, commandIndex) =>
        commandIndex === index ? { ...command, [field]: value } : command,
      ),
    );
  }
  async function importCSV(file?: File) {
    if (!file) return;
    setError("");
    setImportMessage("");
    try {
      const content = (await file.text()).replace(/^\uFEFF/, "");
      const imported: TR069Command[] = [];
      const seen = new Set<string>();
      for (const [index, rawRow] of content.split(/\r?\n/).entries()) {
        if (!rawRow.trim()) continue;
        const fields = parseCSVRow(rawRow);
        if (fields.length !== 2 || !fields[0] || !fields[1]) {
          throw new Error(
            `Linha ${index + 1} inválida. Use o formato comando,tipo.`,
          );
        }
        const [parameterPath, valueType] = fields;
        if (!tr069ValueTypes.includes(valueType)) {
          throw new Error(
            `Tipo “${valueType}” inválido na linha ${index + 1}.`,
          );
        }
        const key = parameterPath.toLowerCase();
        if (seen.has(key)) {
          throw new Error(`Comando duplicado na linha ${index + 1}.`);
        }
        seen.add(key);
        imported.push({
          parameter_path: parameterPath,
          value_type: valueType,
          active: true,
        });
      }
      if (!imported.length) throw new Error("O arquivo CSV está vazio.");
      const currentCommands = commands.filter((command) =>
        command.parameter_path.trim(),
      );
      const currentKeys = new Set(
        currentCommands.map((command) => command.parameter_path.toLowerCase()),
      );
      const duplicate = imported.find((command) =>
        currentKeys.has(command.parameter_path.toLowerCase()),
      );
      if (duplicate) {
        throw new Error(
          `O comando “${duplicate.parameter_path}” já está cadastrado.`,
        );
      }
      setCommands([...currentCommands, ...imported]);
      setImportMessage(
        `${imported.length} comando${imported.length === 1 ? " importado" : "s importados"}. Salve para gravar no banco.`,
      );
    } catch (importError) {
      setError(
        importError instanceof Error
          ? importError.message
          : "Não foi possível importar o CSV.",
      );
    }
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await api(
        item
          ? `/api/v1/acs/tr069-data-models/${item.id}`
          : "/api/v1/acs/tr069-data-models",
        {
          method: item ? "PUT" : "POST",
          body: JSON.stringify({ name, active, commands }),
        },
      );
      onSaved();
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : "Não foi possível salvar.",
      );
    } finally {
      setSaving(false);
    }
  }
  return (
    <CatalogModal
      title={item ? "Editar modelo de dados TR-069" : "Adicionar modelo de dados TR-069"}
      description="Cadastre o nome e os parâmetros que o ACS deve coletar."
      saving={saving}
      error={error}
      onClose={onClose}
      onSubmit={submit}
    >
      <Field label="Nome">
        <input
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Ex.: tr-181"
          maxLength={100}
          required
          autoFocus
        />
      </Field>
      <div className="option-group">
        <span>Comandos / parâmetros</span>
        <label className="file-picker">
          <Upload size={16} />
          <span>Importar CSV sem cabeçalho (comando,tipo)</span>
          <input
            type="file"
            accept=".csv,text/csv,text/plain"
            onChange={(event) => {
              void importCSV(event.target.files?.[0]);
              event.target.value = "";
            }}
          />
        </label>
        {importMessage && <p className="import-status">{importMessage}</p>}
        {commands.map((command, index) => (
          <div className="form-grid" key={command.id || index}>
            <Field label="Caminho">
              <input
                value={command.parameter_path}
                onChange={(event) =>
                  updateCommand(index, "parameter_path", event.target.value)
                }
                placeholder="Device.DeviceInfo.HardwareVersion"
                maxLength={500}
                required
              />
            </Field>
            <Field label="Tipo">
              <select
                value={command.value_type}
                onChange={(event) =>
                  updateCommand(index, "value_type", event.target.value)
                }
              >
                {tr069ValueTypes.map((valueType) => (
                  <option key={valueType} value={valueType}>
                    {valueType}
                  </option>
                ))}
              </select>
            </Field>
            <button
              className="button danger-button"
              type="button"
              title="Remover comando"
              onClick={() =>
                setCommands((current) =>
                  current.filter((_, commandIndex) => commandIndex !== index),
                )
              }
            >
              <Trash2 size={16} /> Remover
            </button>
          </div>
        ))}
        <button
          className="button"
          type="button"
          onClick={() =>
            setCommands((current) => [
              ...current,
              { parameter_path: "", value_type: "string", active: true },
            ])
          }
        >
          <Plus size={16} /> Adicionar comando
        </button>
      </div>
      <div className="check-row">
        <label>
          <input
            type="checkbox"
            checked={active}
            onChange={(event) => setActive(event.target.checked)}
          />{" "}
          Ativo
        </label>
      </div>
    </CatalogModal>
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
  dataModels,
  onClose,
  onSaved,
}: {
  item: Model | null;
  initial?: Partial<Model>;
  manufacturers: Manufacturer[];
  dataModels: TR069DataModel[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const source = item || initial;
  const [manufacturerId, setManufacturer] = useState(
      source?.manufacturer_id || "",
    ),
    [dataModelId, setDataModelId] = useState(
      source?.tr069_data_model_id || "",
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
      tr069_data_model_id: dataModelId,
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
        <Field label="Modelo de dados TR-069">
          <select
            value={dataModelId}
            onChange={(event) => setDataModelId(event.target.value)}
            required
          >
            <option value="">Selecione</option>
            {dataModels
              .filter(
                (dataModel) =>
                  dataModel.active || dataModel.id === dataModelId,
              )
              .map((dataModel) => (
                <option key={dataModel.id} value={dataModel.id}>
                  {dataModel.name}
                </option>
              ))}
          </select>
        </Field>
      </div>
      <div className="form-grid">
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
