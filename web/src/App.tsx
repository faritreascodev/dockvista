import { useMemo, type ReactNode } from "react";
import { Navigate, Route, BrowserRouter, Routes } from "react-router-dom";
import { acceptInvite } from "./api/client";
import { AuthScreen } from "./components/AuthScreen";
import { Logo } from "./components/Logo";
import { Shell } from "./components/Shell";
import { ToastProvider } from "./components/ui/Toast";
import { useAuth } from "./hooks/useAuth";
import { EventsProvider } from "./hooks/useEventsContext";
import { FleetStatsProvider } from "./hooks/useFleetStatsContext";
import { SearchProvider } from "./hooks/useSearchContext";
import { EnvironmentProvider, useEnvironments } from "./hooks/useEnvironments";
import { SessionContext } from "./hooks/useSession";
import { ComposePage } from "./pages/ComposePage";
import { ContainersPage } from "./pages/ContainersPage";
import { ImagesPage } from "./pages/ImagesPage";
import { NetworksPage } from "./pages/NetworksPage";
import { OverviewPage } from "./pages/OverviewPage";
import { SettingsPage } from "./pages/SettingsPage";
import { StoragePage } from "./pages/StoragePage";
import { SwarmPage } from "./pages/SwarmPage";
import { VolumesPage } from "./pages/VolumesPage";

function inviteTokenFromLocation(): string | null {
  const url = new URL(window.location.href);
  if (url.pathname !== "/invite") return null;
  const token = url.searchParams.get("token");
  return token && token.length > 0 ? token : null;
}

function LiveScope({ children }: { children: ReactNode }) {
  const { generation } = useEnvironments();
  return (
    <EventsProvider key={generation}>
      <FleetStatsProvider key={generation}>{children}</FleetStatsProvider>
    </EventsProvider>
  );
}

export default function App() {
  const { state: auth, setup, login, logout } = useAuth();
  const session = useMemo(
    () =>
      auth.phase === "authenticated"
        ? {
            username: auth.username,
            role: auth.role,
            readOnly: auth.readOnly,
            instanceReadOnly: auth.instanceReadOnly,
          }
        : { username: "", role: "viewer" as const, readOnly: false, instanceReadOnly: false },
    [auth],
  );

  if (auth.phase === "loading") {
    return (
      <div className="flex h-screen items-center justify-center bg-canvas">
        <Logo className="h-9 w-9 animate-pulse" />
      </div>
    );
  }
  if (auth.phase === "needs-setup") {
    return <AuthScreen mode="setup" onSubmit={setup} />;
  }
  const inviteToken = inviteTokenFromLocation();
  if (auth.phase === "needs-login" && inviteToken) {
    return (
      <AuthScreen
        mode="invite"
        inviteToken={inviteToken}
        onSubmit={async (username, password) => {
          await acceptInvite(inviteToken, username, password);
          await login(username, password);
        }}
      />
    );
  }
  if (auth.phase === "needs-login") {
    return <AuthScreen mode="login" onSubmit={login} />;
  }

  return (
    <SessionContext.Provider value={session}>
      <ToastProvider>
        <EnvironmentProvider>
          <LiveScope>
            <SearchProvider>
              <BrowserRouter>
                <Routes>
                  <Route element={<Shell onLogout={logout} />}>
                    <Route index element={<Navigate to="/overview" replace />} />
                    <Route path="/overview" element={<OverviewPage />} />
                    <Route path="/containers" element={<ContainersPage />} />
                    <Route path="/images" element={<ImagesPage />} />
                    <Route path="/volumes" element={<VolumesPage />} />
                    <Route path="/networks" element={<NetworksPage />} />
                    <Route path="/storage" element={<StoragePage />} />
                    <Route path="/compose" element={<ComposePage />} />
                    <Route path="/swarm" element={<SwarmPage />} />
                    <Route path="/settings" element={<SettingsPage />} />
                    <Route path="*" element={<Navigate to="/overview" replace />} />
                  </Route>
                </Routes>
              </BrowserRouter>
            </SearchProvider>
          </LiveScope>
        </EnvironmentProvider>
      </ToastProvider>
    </SessionContext.Provider>
  );
}
