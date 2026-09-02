import { Navigate, Route, BrowserRouter, Routes } from "react-router-dom";
import { AuthScreen } from "./components/AuthScreen";
import { Shell } from "./components/Shell";
import { ToastProvider } from "./components/ui/Toast";
import { useAuth } from "./hooks/useAuth";
import { EventsProvider } from "./hooks/useEventsContext";
import { SearchProvider } from "./hooks/useSearchContext";
import { ComposePage } from "./pages/ComposePage";
import { ContainersPage } from "./pages/ContainersPage";
import { ImagesPage } from "./pages/ImagesPage";
import { NetworksPage } from "./pages/NetworksPage";
import { SettingsPage } from "./pages/SettingsPage";
import { VolumesPage } from "./pages/VolumesPage";

export default function App() {
  const { state: auth, setup, login, logout } = useAuth();

  if (auth.phase === "loading") {
    return <div className="flex h-screen items-center justify-center bg-surface-950" />;
  }
  if (auth.phase === "needs-setup") {
    return <AuthScreen mode="setup" onSubmit={setup} />;
  }
  if (auth.phase === "needs-login") {
    return <AuthScreen mode="login" onSubmit={login} />;
  }

  return (
    <ToastProvider>
      <EventsProvider>
        <SearchProvider>
          <BrowserRouter>
            <Routes>
              <Route element={<Shell username={auth.username} onLogout={logout} />}>
                <Route index element={<Navigate to="/containers" replace />} />
                <Route path="/containers" element={<ContainersPage />} />
                <Route path="/images" element={<ImagesPage />} />
                <Route path="/volumes" element={<VolumesPage />} />
                <Route path="/networks" element={<NetworksPage />} />
                <Route path="/compose" element={<ComposePage />} />
                <Route path="/settings" element={<SettingsPage />} />
                <Route path="*" element={<Navigate to="/containers" replace />} />
              </Route>
            </Routes>
          </BrowserRouter>
        </SearchProvider>
      </EventsProvider>
    </ToastProvider>
  );
}
