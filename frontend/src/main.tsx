import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/AuthContext";
import { GuestRoute, ProtectedRoute } from "./auth/ProtectedRoute";
import AppLayout from "./layout/AppLayout";
import MenuGuard from "./layout/MenuGuard";
import BranchPage from "./pages/BranchPage";
import Dashboard from "./pages/Dashboard";
import Login from "./pages/Login";
import ObatPage from "./pages/ObatPage";
import MenuPage from "./pages/MenuPage";
import Register from "./pages/Register";
import RolePage from "./pages/RolePage";
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
            {/* Routes from the menus table that have no page yet. */}
            <Route path="*" element={<MenuPage />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
