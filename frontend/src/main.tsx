import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/AuthContext";
import { GuestRoute, ProtectedRoute } from "./auth/ProtectedRoute";
import AppLayout from "./layout/AppLayout";
import MenuGuard from "./layout/MenuGuard";
import BranchPage from "./pages/BranchPage";
import Dashboard from "./pages/Dashboard";
import DistributorPage from "./pages/DistributorPage";
import DokterPage from "./pages/DokterPage";
import IcdPage from "./pages/IcdPage";
import JamkesPage from "./pages/JamkesPage";
import Login from "./pages/Login";
import ObatPage from "./pages/ObatPage";
import PasienPage from "./pages/PasienPage";
import PendaftaranPage from "./pages/PendaftaranPage";
import OrganisasiLokasiPage from "./pages/OrganisasiLokasiPage";
import PerujukOtcPage from "./pages/PerujukOtcPage";
import PrincipalPage from "./pages/PrincipalPage";
import PenunjangPage from "./pages/PenunjangPage";
import MenuPage from "./pages/MenuPage";
import Register from "./pages/Register";
import RolePage from "./pages/RolePage";
import TindakanPage from "./pages/TindakanPage";
import UserPage from "./pages/UserPage";
import "./styles.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/register" element={<GuestRoute><Register /></GuestRoute>} />
          <Route path="/login" element={<GuestRoute><Login /></GuestRoute>} />
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          {/* Every page after login shares the sidebar layout. */}
          <Route element={<ProtectedRoute><AppLayout /></ProtectedRoute>}>
            <Route path="/dashboard" element={<Dashboard />} />
            {/* Built pages: only open when the user's role grants the menu item. */}
            <Route path="/management/role" element={<MenuGuard><RolePage /></MenuGuard>} />
            <Route path="/management/user" element={<MenuGuard><UserPage /></MenuGuard>} />
            <Route path="/management/cabang" element={<MenuGuard><BranchPage /></MenuGuard>} />
            <Route path="/master-data-klinik/obat" element={<MenuGuard><ObatPage /></MenuGuard>} />
            <Route path="/master-data-klinik/tindakan" element={<MenuGuard><TindakanPage /></MenuGuard>} />
            <Route path="/master-data-klinik/penunjang" element={<MenuGuard><PenunjangPage /></MenuGuard>} />
            <Route path="/master-data-klinik/pasien" element={<MenuGuard><PasienPage /></MenuGuard>} />
            <Route path="/master-data-klinik/dokter" element={<MenuGuard><DokterPage /></MenuGuard>} />
            <Route path="/master-data-klinik/jaminan-kesehatan" element={<MenuGuard><JamkesPage /></MenuGuard>} />
            <Route path="/master-data-klinik/distributor" element={<MenuGuard><DistributorPage /></MenuGuard>} />
            <Route path="/master-data-klinik/principal" element={<MenuGuard><PrincipalPage /></MenuGuard>} />
            <Route path="/master-data-klinik/icd" element={<MenuGuard><IcdPage /></MenuGuard>} />
            <Route path="/master-data-klinik/penjual-otc" element={<MenuGuard><PerujukOtcPage /></MenuGuard>} />
            <Route path="/front-office-klinik/pendaftaran-pasien" element={<MenuGuard><PendaftaranPage /></MenuGuard>} />
            <Route path="/master-data-satu-sehat/organisasi-lokasi" element={<MenuGuard><OrganisasiLokasiPage /></MenuGuard>} />
            {/* Routes from the menus table that have no page yet. */}
            <Route path="*" element={<MenuPage />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
