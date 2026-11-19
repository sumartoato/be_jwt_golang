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

/** A reftindakandetail row: one procedure's prices at one branch. */
export interface TindakanPrice {
  cabang: string;
  hbeli: number | null;
  hjual: number | null;
  /** Price for health-insurance (jaminan kesehatan) patients. */
  hjualJamkes: number | null;
  isActive: boolean;
}

/** A reftindakan row. */
export interface Tindakan {
  id: number;
  nama: string;
  idJenis: number | null;
  hbeli: number | null;
  hjual: number | null;
  hjualJamkes: number | null;
  isActive: boolean;
  isEdit: boolean;
  cetakKartu: boolean;
  updatedBy: string;
  updatedAt: string | null;
  /** Only for the branches the user is mapped to. */
  prices: TindakanPrice[];
}

export type TindakanInput = Omit<Tindakan, "id" | "updatedBy" | "updatedAt" | "prices">;
export type TindakanPriceInput = Omit<TindakanPrice, "cabang">;

/** A refpenunjangdetail row: one examination's prices at one branch (lab has no purchase price). */
export interface PenunjangPrice {
  cabang: string;
  hjual: number | null;
  /** Price for health-insurance (jaminan kesehatan) patients. */
  hjualJamkes: number | null;
  isActive: boolean;
}

/** A refpenunjang row (supporting examination: lab, radiology, ...). */
export interface Penunjang {
  id: number;
  nama: string;
  idJenis: number | null;
  hjual: number | null;
  hjualJamkes: number | null;
  isActive: boolean;
  updatedBy: string;
  updatedAt: string | null;
  /** Only for the branches the user is mapped to. */
  prices: PenunjangPrice[];
}

export type PenunjangInput = Omit<Penunjang, "id" | "updatedBy" | "updatedAt" | "prices">;
export type PenunjangPriceInput = Omit<PenunjangPrice, "cabang">;

/** A patient as listed (refmpasien). Dates are YYYY-MM-DD. */
export interface PasienRow {
  id: number;
  noRm: string;
  nama: string;
  tmptLahir: string;
  tglLahir: string | null;
  alamat: string;
  rt: string;
  rw: string;
  cabang: string;
}

/** The full patient record for the edit form. */
export interface Pasien extends PasienRow {
  idKel: number | null;
  kelurahan: string;
  kecamatan: string;
  kabupaten: string;
  provinsi: string;
  telp: string;
  hp: string;
  idGender: number | null;
  idGolDarah: number | null;
  idAgama: number | null;
  idStatusKwn: number | null;
  /** First visit date. */
  tglKun: string | null;
  penanggung: string;
  idJamkes: number | null;
  noJamkes: string;
  nik: string;
  alergi: string;
  ihsId: string;
  updatedBy: string;
  updatedAt: string | null;
}

export type PasienInput = Pick<
  Pasien,
  | "nama" | "tmptLahir" | "tglLahir" | "alamat" | "idKel" | "rt" | "rw" | "telp" | "hp" | "idGender" | "idGolDarah"
  | "idAgama" | "idStatusKwn" | "tglKun" | "penanggung" | "idJamkes" | "noJamkes" | "cabang" | "nik" | "alergi"
>;

export interface PasienOption {
  id: number;
  name: string;
  isActive: boolean;
}

export interface PasienOptions {
  gender: PasienOption[];
  golDarah: PasienOption[];
  agama: PasienOption[];
  statusKwn: PasienOption[];
  jamkes: PasienOption[];
  branches: { kode: string; nama: string }[];
}

export interface KelurahanHit {
  id: number;
  kelurahan: string;
  kecamatan: string;
  kabupaten: string;
  provinsi: string;
}

export interface PasienPage {
  rows: PasienRow[];
  total: number;
  unfilteredTotal: number;
  branches: { kode: string; nama: string }[];
}

/** A doctor (refmdokter) with the codes of its practice branches. */
export interface Dokter {
  id: number;
  nama: string;
  idJenis: number | null;
  tglMasuk: string | null;
  tmptLahir: string;
  tglLahir: string | null;
  alamat: string;
  telp: string;
  hp: string;
  pendidikan: string;
  spesialis: string;
  idGender: number | null;
  idAgama: number | null;
  idGolDarah: number | null;
  idStatusKwn: number | null;
  nik: string;
  isActive: boolean;
  cabangPraktek: string[];
  updatedBy: string;
  updatedAt: string | null;
}

export type DokterInput = Omit<Dokter, "id" | "cabangPraktek" | "updatedBy" | "updatedAt">;

export interface DokterOptions {
  jenis: LookupOption[];
  gender: LookupOption[];
  agama: LookupOption[];
  golDarah: LookupOption[];
  statusKwn: LookupOption[];
}

export interface DokterTindakanItem {
  id: number;
  nama: string;
  jenis: string;
}

export interface DokterTindakanLists {
  assigned: DokterTindakanItem[];
  available: DokterTindakanItem[];
}

export interface DokterCabangLists {
  assigned: { kode: string; nama: string }[];
  available: { kode: string; nama: string }[];
  /** Branches the user may add or remove (their own). */
  editable: string[];
}

/** A health insurer (refjaminankesehatan). */
export interface Jamkes {
  id: number;
  nama: string;
  alamat: string;
  telp: string;
  fax: string;
  /** refjaminankesehatantype (PT, CV, Non PT CV, ...). */
  idType: number | null;
  isActive: boolean;
  /** status_limit 1 = Limit, 2 = No Limit. */
  hasLimit: boolean;
  limit: number | null;
  updatedBy: string;
  updatedAt: string | null;
}

export type JamkesInput = Omit<Jamkes, "id" | "updatedBy" | "updatedAt">;

/** A PKS (agreement) term of an insurer; every term of the master list is returned. */
export interface JamkesTerm {
  detailId: number;
  description: string;
  value: string;
  shown: boolean;
  saved: boolean;
}

export interface JamkesItem {
  no: number;
  value: string;
  shown: boolean;
}

/** A drug distributor (refdistributorobat). */
export interface Distributor {
  id: number;
  nama: string;
  alamat: string;
  telp: string;
  fax: string;
  /** Contact person. */
  cp: string;
  /** Contact person's phone. */
  telpCp: string;
  updatedBy: string;
  updatedAt: string | null;
}

export type DistributorInput = Pick<Distributor, "nama" | "alamat" | "telp" | "fax" | "cp" | "telpCp">;

/** A drug principal (refprincipalobat). */
export interface Principal {
  id: number;
  nama: string;
}

/** An ICD-X diagnosis code (reficd). */
export interface Icd {
  id: number;
  kode: string;
  nama: string;
  idJenis: number;
  /** refjenisicd name (GIGI, THT, ...). */
  jenis: string;
  updatedBy: string;
  updatedAt: string | null;
}

export interface IcdInput {
  kode: string;
  nama: string;
  idJenis: number | null;
}

/** A person who refers over-the-counter sales (refperujukotc). */
export interface PerujukOtc {
  id: number;
  nama: string;
}

/** SATUSEHAT Organization and Location of a branch's poli (reforganizationlocation). */
export interface OrganisasiLokasi {
  id: number;
  cabang: string;
  namaCabang: string;
  idJenisRawat: number | null;
  poli: string;
  organisasi: string;
  lokasi: string;
  /** SATUSEHAT ids (UUID); "" until registered. */
  organisasiId: string;
  lokasiId: string;
  updatedBy: string;
  updatedAt: string | null;
}

export type OrganisasiLokasiInput = Pick<OrganisasiLokasi, "organisasi" | "lokasi" | "organisasiId" | "lokasiId">;

/** A visit registration (trxpendaftaran) with its patient. */
export interface Pendaftaran {
  id: number;
  noAntri: number | null;
  /** e.g. U015; null for services without a poli queue. */
  noAntriPoli: string | null;
  /** "YYYY-MM-DD HH:MM:SS", clinic time. */
  tglMasuk: string | null;
  idJenis: number | null;
  pelayanan: string;
  idStatus: number | null;
  status: string;
  idPas: number | null;
  /** "" when the patient no longer exists. */
  noRm: string;
  nama: string;
  tglLahir: string | null;
  alamat: string;
  cabang: string;
  pakaiKartu: boolean;
  totalPoint: number | null;
  cancelBy: string;
  cancelAt: string | null;
  cancelReason: string;
}

export interface PendaftaranService {
  id: number;
  name: string;
  needQueue: boolean;
  firstCode: string;
  isExamination: boolean;
}

export interface PendaftaranOptions {
  pelayanan: PendaftaranService[];
  status: LookupOption[];
  branches: { kode: string; nama: string }[];
  /** Today in the clinic's time zone. */
  today: string;
}

export interface PendaftaranPasien {
  id: number;
  noRm: string;
  nama: string;
  tglLahir: string | null;
  alamat: string;
  nik: string;
  hp: string;
  penanggung: string;
  jamkes: string;
  cabang: string;
}

/** Visit status ids (refstatustransaksi). */
export const STATUS_DAFTAR = 3;

/** Settlement status of a sale (trxjualtotal.status: 1 close, 2 cancel, else open). */
export type PengobatanStatus = "open" | "close" | "cancel";

/** Transaksi Klinik > Pengobatan: one treatment sale (trxjual + trxjualtotal). */
export interface Pengobatan {
  id: number;
  /** "YYYY-MM-DD HH:MM:SS", clinic time. */
  tgl: string | null;
  noKwi: string;
  idPas: number | null;
  noRm: string;
  /** Empty when the patient no longer exists. */
  pasien: string;
  dokter: string;
  idJenisRawat: number | null;
  jenisRawat: string;
  idJenisBayar: number | null;
  pembayaran: string;
  totalObat: number;
  totalTindakan: number;
  totalPenunjang: number;
  potongan: number;
  total: number;
  cabang: string;
  status: PengobatanStatus;
  cancelBy: string;
  cancelAt: string | null;
  cancelReason: string;
}

export interface PengobatanOptions {
  pelayanan: LookupOption[];
  statusKunjungan: LookupOption[];
  pembayaran: LookupOption[];
  /** Visit statuses a treatment can still be recorded for (DAFTAR, PROSES, PAYMENT). */
  proses: LookupOption[];
  branches: { kode: string; nama: string }[];
  /** Today in the clinic's time zone. */
  today: string;
}

/** A registration offered by the Pengobatan form's "Data Pendaftaran" lookup. */
export interface PengobatanPendaftaran {
  id: number;
  tglMasuk: string | null;
  idJenisRawat: number | null;
  pelayanan: string;
  isPemeriksaan: boolean;
  idStatus: number | null;
  status: string;
  idPas: number | null;
  noRm: string;
  nama: string;
  tglLahir: string | null;
  alamat: string;
  hp: string;
  penanggung: string;
  idJamkes: number | null;
  jamkes: string;
  noJamkes: string;
  cabang: string;
  /** Dokter PJ; null when not chosen yet. */
  idDokter: number | null;
  dokter: string;
}

/** A doctor in the Pengobatan form's Dokter PJ lookup. */
export interface PengobatanDokter {
  id: number;
  nama: string;
  /** refjenisdokter; empty when not set. */
  jenis: string;
}

/** A drug in the Pengobatan form's "Data Obat" lookup, with the branch price and stock. */
export interface PengobatanObat {
  id: number;
  nama: string;
  jenis: string;
  satuan: string;
  harga: number;
  stok: number;
  kenaPajak: boolean;
}

export interface PengobatanPendaftaranQuery {
  cabang: string;
  from: string;
  to: string;
  page: number;
  size: number;
  nama?: string;
  pelayanan?: number;
  status?: number;
  q?: string;
  sort?: string;
  dir?: string;
}

export interface PengobatanQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  pelayanan?: number;
  statusKunjungan?: number;
  pembayaran?: number;
  status?: PengobatanStatus;
}

/** Fields of the patient "Pencarian" panel in the registration form. */
export type PasienSearchField = "norm" | "nama" | "umur" | "alamat" | "jamkes";

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

  // Master Data Klinik > Tindakan
  tindakan: (token: string) =>
    request<{ tindakan: Tindakan[]; branches: { kode: string; nama: string }[] }>("/api/tindakan", { token }),
  tindakanOptions: (token: string) => request<{ jenis: LookupOption[] }>("/api/tindakan/options", { token }),
  createTindakan: (token: string, input: TindakanInput) =>
    request<Tindakan>("/api/tindakan", { method: "POST", body: input, token }),
  updateTindakan: (token: string, id: number, input: TindakanInput) =>
    request<Tindakan>(`/api/tindakan/${id}`, { method: "PUT", body: input, token }),
  setTindakanPrice: (token: string, id: number, cabang: string, input: TindakanPriceInput) =>
    request<Tindakan>(`/api/tindakan/${id}/price/${encodeURIComponent(cabang)}`, { method: "PUT", body: input, token }),

  // Master Data Klinik > Penunjang
  penunjang: (token: string) =>
    request<{ penunjang: Penunjang[]; branches: { kode: string; nama: string }[] }>("/api/penunjang", { token }),
  penunjangOptions: (token: string) => request<{ jenis: LookupOption[] }>("/api/penunjang/options", { token }),
  createPenunjang: (token: string, input: PenunjangInput) =>
    request<Penunjang>("/api/penunjang", { method: "POST", body: input, token }),
  updatePenunjang: (token: string, id: number, input: PenunjangInput) =>
    request<Penunjang>(`/api/penunjang/${id}`, { method: "PUT", body: input, token }),
  setPenunjangPrice: (token: string, id: number, cabang: string, input: PenunjangPriceInput) =>
    request<Penunjang>(`/api/penunjang/${id}/price/${encodeURIComponent(cabang)}`, { method: "PUT", body: input, token }),

  // Master Data Klinik > Pasien (server-side paging)
  pasien: (token: string, params: { page: number; size: number; q?: string; sort?: string; dir?: string; cabang?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<PasienPage>(`/api/pasien?${qs}`, { token });
  },
  pasienOptions: (token: string) => request<PasienOptions>("/api/pasien/options", { token }),
  pasienDetail: (token: string, id: number) => request<Pasien>(`/api/pasien/${id}`, { token }),
  searchKelurahan: (token: string, q: string) =>
    request<{ kelurahan: KelurahanHit[] }>(`/api/pasien/kelurahan?q=${encodeURIComponent(q)}`, { token }),
  createPasien: (token: string, input: PasienInput) => request<Pasien>("/api/pasien", { method: "POST", body: input, token }),
  updatePasien: (token: string, id: number, input: PasienInput) =>
    request<Pasien>(`/api/pasien/${id}`, { method: "PUT", body: input, token }),

  // Master Data Klinik > Dokter
  dokter: (token: string) => request<{ dokter: Dokter[]; branches: { kode: string; nama: string }[] }>("/api/dokter", { token }),
  dokterOptions: (token: string) => request<DokterOptions>("/api/dokter/options", { token }),
  createDokter: (token: string, input: DokterInput) => request<Dokter>("/api/dokter", { method: "POST", body: input, token }),
  updateDokter: (token: string, id: number, input: DokterInput) =>
    request<Dokter>(`/api/dokter/${id}`, { method: "PUT", body: input, token }),
  dokterTindakan: (token: string, id: number) => request<DokterTindakanLists>(`/api/dokter/${id}/tindakan`, { token }),
  changeDokterTindakan: (token: string, id: number, change: { add?: number[]; remove?: number[] }) =>
    request<DokterTindakanLists>(`/api/dokter/${id}/tindakan`, { method: "PATCH", body: change, token }),
  dokterCabang: (token: string, id: number) => request<DokterCabangLists>(`/api/dokter/${id}/cabang`, { token }),
  changeDokterCabang: (token: string, id: number, change: { add?: string[]; remove?: string[] }) =>
    request<DokterCabangLists>(`/api/dokter/${id}/cabang`, { method: "PATCH", body: change, token }),

  // Master Data Klinik > Jaminan Kesehatan
  jamkes: (token: string) => request<{ jamkes: Jamkes[]; types: LookupOption[] }>("/api/jamkes", { token }),
  createJamkes: (token: string, input: JamkesInput) => request<Jamkes>("/api/jamkes", { method: "POST", body: input, token }),
  updateJamkes: (token: string, id: number, input: JamkesInput) =>
    request<Jamkes>(`/api/jamkes/${id}`, { method: "PUT", body: input, token }),
  jamkesPks: (token: string, id: number) => request<{ terms: JamkesTerm[] }>(`/api/jamkes/${id}/pks`, { token }),
  saveJamkesPks: (token: string, id: number, terms: { detailId: number; value: string; shown: boolean }[]) =>
    request<{ terms: JamkesTerm[] }>(`/api/jamkes/${id}/pks`, { method: "PUT", body: { terms }, token }),
  jamkesItems: (token: string, id: number) => request<{ items: JamkesItem[] }>(`/api/jamkes/${id}/items`, { token }),
  saveJamkesItems: (token: string, id: number, items: { value: string; shown: boolean }[]) =>
    request<{ items: JamkesItem[] }>(`/api/jamkes/${id}/items`, { method: "PUT", body: { items }, token }),

  // Master Data Klinik > Distributor
  distributor: (token: string) => request<{ distributor: Distributor[] }>("/api/distributor", { token }),
  createDistributor: (token: string, input: DistributorInput) =>
    request<Distributor>("/api/distributor", { method: "POST", body: input, token }),
  updateDistributor: (token: string, id: number, input: DistributorInput) =>
    request<Distributor>(`/api/distributor/${id}`, { method: "PUT", body: input, token }),

  // Master Data Klinik > Principal
  principal: (token: string) => request<{ principal: Principal[] }>("/api/principal", { token }),
  createPrincipal: (token: string, nama: string) => request<Principal>("/api/principal", { method: "POST", body: { nama }, token }),
  updatePrincipal: (token: string, id: number, nama: string) =>
    request<Principal>(`/api/principal/${id}`, { method: "PUT", body: { nama }, token }),

  // Master Data Klinik > ICD (server-side paging)
  icd: (token: string, params: { page: number; size: number; q?: string; sort?: string; dir?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: Icd[]; total: number; unfilteredTotal: number }>(`/api/icd?${qs}`, { token });
  },
  icdOptions: (token: string) => request<{ jenis: LookupOption[] }>("/api/icd/options", { token }),
  createIcd: (token: string, input: IcdInput) => request<Icd>("/api/icd", { method: "POST", body: input, token }),
  updateIcd: (token: string, id: number, input: IcdInput) => request<Icd>(`/api/icd/${id}`, { method: "PUT", body: input, token }),

  // Master Data Klinik > Penjual OTC (Perujuk OTC)
  perujukOtc: (token: string) => request<{ perujuk: PerujukOtc[] }>("/api/perujuk-otc", { token }),
  createPerujukOtc: (token: string, nama: string) => request<PerujukOtc>("/api/perujuk-otc", { method: "POST", body: { nama }, token }),
  updatePerujukOtc: (token: string, id: number, nama: string) =>
    request<PerujukOtc>(`/api/perujuk-otc/${id}`, { method: "PUT", body: { nama }, token }),

  // Master Data Satu Sehat > Organisasi & Lokasi
  organisasiLokasi: (token: string) =>
    request<{ rows: OrganisasiLokasi[]; branches: { kode: string; nama: string }[] }>("/api/organisasi-lokasi", { token }),
  updateOrganisasiLokasi: (token: string, id: number, input: OrganisasiLokasiInput) =>
    request<OrganisasiLokasi>(`/api/organisasi-lokasi/${id}`, { method: "PUT", body: input, token }),

  // Transaksi Klinik > Pengobatan (server-side paging)
  pengobatan: (token: string, params: PengobatanQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: Pengobatan[]; total: number; unfilteredTotal: number }>(`/api/pengobatan?${qs}`, { token });
  },
  pengobatanOptions: (token: string) => request<PengobatanOptions>("/api/pengobatan/options", { token }),
  pengobatanDokter: (token: string, cabang: string) =>
    request<{ rows: PengobatanDokter[] }>(`/api/pengobatan/dokter?cabang=${encodeURIComponent(cabang)}`, { token }),
  pengobatanObat: (token: string, cabang: string) =>
    request<{ rows: PengobatanObat[] }>(`/api/pengobatan/obat?cabang=${encodeURIComponent(cabang)}`, { token }),
  pengobatanPendaftaran: (token: string, params: PengobatanPendaftaranQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PengobatanPendaftaran[]; total: number; unfilteredTotal: number }>(`/api/pengobatan/pendaftaran?${qs}`, { token });
  },

  // Front Office Klinik > Pendaftaran Pasien (server-side paging)
  pendaftaran: (
    token: string,
    params: { from: string; to: string; page: number; size: number; q?: string; sort?: string; dir?: string; cabang?: string; pelayanan?: number; status?: number },
  ) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: Pendaftaran[]; total: number; unfilteredTotal: number }>(`/api/pendaftaran?${qs}`, { token });
  },
  pendaftaranOptions: (token: string) => request<PendaftaranOptions>("/api/pendaftaran/options", { token }),
  pendaftaranPasien: (token: string, params: { q?: string; by?: PasienSearchField | ""; key?: string; page: number; size: number }) => {
    const qs = new URLSearchParams({ page: String(params.page), size: String(params.size), q: params.q ?? "", by: params.by ?? "", key: params.key ?? "" });
    return request<{ rows: PendaftaranPasien[]; total: number; unfilteredTotal: number }>(`/api/pendaftaran/pasien?${qs}`, { token });
  },
  createPendaftaran: (token: string, input: { tanggal: string; idPas: number; idJenis: number; cabang: string; pakaiKartu: boolean }) =>
    request<Pendaftaran>("/api/pendaftaran", { method: "POST", body: input, token }),
  updatePendaftaran: (token: string, id: number, input: { idJenis: number; pakaiKartu: boolean }) =>
    request<Pendaftaran>(`/api/pendaftaran/${id}`, { method: "PUT", body: input, token }),
  cancelPendaftaran: (token: string, id: number, reason: string) =>
    request<Pendaftaran>(`/api/pendaftaran/${id}/cancel`, { method: "POST", body: { reason }, token }),

  // Management > Cabang
  branches: (token: string) => request<{ branches: Branch[] }>("/api/branches", { token }),
  createBranch: (token: string, input: BranchInput) =>
    request<Branch>("/api/branches", { method: "POST", body: input, token }),
  updateBranch: (token: string, kode: string, input: BranchInput) =>
    request<Branch>(`/api/branches/${encodeURIComponent(kode)}`, { method: "PUT", body: input, token }),
};
