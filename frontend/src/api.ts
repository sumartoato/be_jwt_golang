export type FieldErrors = Record<string, string>;

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public fieldErrors: FieldErrors = {},
  ) {
    super(message);
  }
}

export interface TokenResponse {
  token: string;
  expiresAt: string;
}

export interface MeResponse {
  id: string;
  email: string;
  message: string;
}

/** One node of the sidebar menu, configured in the `menus` table. */
export interface MenuItem {
  id: number;
  title: string;
  icon: string;
  /** Route to open, or null for a group that only expands its children. */
  path: string | null;
  children: MenuItem[];
}

export interface Role {
  id: number;
  name: string;
  /** Every active menu, including menus added later. */
  allAccess: boolean;
  /** Granted menus. A group id grants everything under it. */
  menuIds: number[];
  userCount: number;
}

export type RoleInput = Pick<Role, "name" | "allAccess" | "menuIds">;

export interface UserWithRole {
  id: string;
  email: string;
  roleId: number | null;
  /** Branch codes the user belongs to (refbranchauth); a user can have several. */
  branches: string[];
}

export interface BranchOption {
  kode: string;
  nama: string;
  isActive: boolean;
}

/** Clinic branch (Management > Cabang): a row of refbranch. "" stands for NULL. */
export interface Branch {
  id: number;
  /** Primary key; cannot change after creation. */
  kode: string;
  nama: string;
  alamat: string;
  telp: string;
  /** kode_kwt; may differ from kode (e.g. MED uses BTA). */
  kodeKwt: string;
  npwp: string;
  namaNpwp: string;
  dbHbee: string;
  kodeFasyankes: string;
  namaFasyankes: string;
  clientKey: string;
  /** The secret key itself is never sent to the browser. */
  secretKeySet: boolean;
  /** Last 4 characters of the secret key, or "" */
  secretKeyHint: string;
  organizationId: string;
  isActive: boolean;
  /** Own clinic (true) or partner (false). */
  isInternal: boolean;
}

export type BranchInput = Omit<Branch, "id" | "secretKeySet" | "secretKeyHint"> & {
  /** On update, "" keeps the stored key. */
  secretKey: string;
};

/** A trxstock row: one drug's stock and prices at one branch. */
export interface ObatStock {
  cabang: string;
  stok: number | null;
  stokMin: number | null;
  hbeli: number | null;
  hp: number | null;
  hjual: number | null;
  isActive: boolean;
}

/** A refobat row. Lookup columns are ids; names come from api.obatOptions. */
export interface Obat {
  id: number;
  nama: string;
  isi: string;
  isiPerBox: string;
  idJenis: number | null;
  idLogo: number | null;
  idProd: number | null;
  satBeli: number | null;
  satJual: number | null;
  idMoving: number | null;
  idMovingPo: number | null;
  idDistributor: number | null;
  idPrincipal: number | null;
  idGolongan: number | null;
  qjb: number | null;
  hbeli: number | null;
  hjual: number | null;
  hp: number | null;
  nonAktif: boolean;
  kenaPajak: boolean;
  komposisi: string;
  updatedBy: string;
  updatedAt: string | null;
  /** Only for the branches the user is mapped to. */
  stocks: ObatStock[];
}

export type ObatInput = Omit<Obat, "id" | "updatedBy" | "updatedAt" | "stocks">;
export type ObatStockInput = Omit<ObatStock, "cabang">;

export interface LookupOption {
  id: number;
  name: string;
}

export interface ObatOptions {
  jenis: LookupOption[];
  satuan: LookupOption[];
  logo: LookupOption[];
  produsen: LookupOption[];
  moving: LookupOption[];
  movingPo: LookupOption[];
  distributor: LookupOption[];
  principal: LookupOption[];
  golongan: LookupOption[];
}

async function request<T>(path: string, options: { method?: string; body?: unknown; token?: string | null } = {}): Promise<T> {
  const headers: Record<string, string> = {};
  if (options.body !== undefined) headers["Content-Type"] = "application/json";
  if (options.token) headers["Authorization"] = `Bearer ${options.token}`;

  const res = await fetch(path, {
    method: options.method ?? "GET",
    headers,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError(res.status, data.error ?? `Request failed (${res.status})`, data.errors ?? {});
  }
  return data as T;
}

export const api = {
  register: (email: string, password: string, confirmPassword: string) =>
    request<{ id: string; email: string }>("/api/auth/register", {
      method: "POST",
      body: { email, password, confirmPassword },
    }),
  login: (email: string, password: string) =>
    request<TokenResponse>("/api/auth/login", { method: "POST", body: { email, password } }),
  refresh: (token: string) => request<TokenResponse>("/api/auth/refresh", { method: "POST", token }),
  me: (token: string) => request<MeResponse>("/api/me", { token }),
  /** Menus the caller's role can open, and the role's name. */
  menus: (token: string) => request<{ menus: MenuItem[]; role: string | null }>("/api/menus", { token }),

  // Management > Role
  roles: (token: string) => request<{ roles: Role[]; menus: MenuItem[] }>("/api/roles", { token }),
  createRole: (token: string, input: RoleInput) => request<Role>("/api/roles", { method: "POST", body: input, token }),
  updateRole: (token: string, id: number, input: RoleInput) =>
    request<Role>(`/api/roles/${id}`, { method: "PUT", body: input, token }),
  deleteRole: (token: string, id: number) => request<unknown>(`/api/roles/${id}`, { method: "DELETE", token }),

  // Management > User
  users: (token: string) =>
    request<{ users: UserWithRole[]; roles: { id: number; name: string }[]; branches: BranchOption[] }>("/api/users", { token }),
  setUserRole: (token: string, userId: string, roleId: number | null) =>
    request<{ id: string; roleId: number | null }>(`/api/users/${userId}/role`, { method: "PUT", body: { roleId }, token }),
  /** The menu tree a user gets from their role (what their sidebar shows). */
  userMenus: (token: string, userId: string) =>
    request<{ role: string | null; allAccess: boolean; menus: MenuItem[] }>(`/api/users/${userId}/menus`, { token }),
  setUserBranches: (token: string, userId: string, branches: string[]) =>
    request<{ id: string; branches: string[] }>(`/api/users/${userId}/branches`, { method: "PUT", body: { branches }, token }),

  // Master Data Klinik > Obat
  obat: (token: string) => request<{ obat: Obat[]; branches: { kode: string; nama: string }[] }>("/api/obat", { token }),
  obatOptions: (token: string) => request<ObatOptions>("/api/obat/options", { token }),
  createObat: (token: string, input: ObatInput) => request<Obat>("/api/obat", { method: "POST", body: input, token }),
  updateObat: (token: string, id: number, input: ObatInput) =>
    request<Obat>(`/api/obat/${id}`, { method: "PUT", body: input, token }),
  setObatStock: (token: string, id: number, cabang: string, input: ObatStockInput) =>
    request<Obat>(`/api/obat/${id}/stock/${encodeURIComponent(cabang)}`, { method: "PUT", body: input, token }),

  // Management > Cabang
  branches: (token: string) => request<{ branches: Branch[] }>("/api/branches", { token }),
  createBranch: (token: string, input: BranchInput) =>
    request<Branch>("/api/branches", { method: "POST", body: input, token }),
  updateBranch: (token: string, kode: string, input: BranchInput) =>
    request<Branch>(`/api/branches/${encodeURIComponent(kode)}`, { method: "PUT", body: input, token }),
};
