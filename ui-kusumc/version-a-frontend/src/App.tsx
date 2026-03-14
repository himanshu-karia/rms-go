import { Navigate, Route, Routes } from 'react-router-dom';

import { Layout } from './components/Layout';
import { RequireAuth } from './components/RequireAuth';
import { DashboardPage } from './pages/DashboardPage';
import { DeviceEnrollmentPage } from './pages/DeviceEnrollmentPage';
import {
  DeviceDriveConfigPage,
  DeviceGovernmentCredentialsPage,
  DeviceInternalCredentialsPage,
} from './pages/DeviceConfigurationPage';
import { DeviceImportPage } from './pages/DeviceImportPage';
import { DeviceImportJobsPage } from './pages/DeviceImportJobsPage';
import { AdminStatesPage } from './pages/AdminStatesPage';
import { AdminStateAuthoritiesPage } from './pages/AdminStateAuthoritiesPage';
import { AdminProjectsPage } from './pages/AdminProjectsPage';
import { AdminServerVendorsPage } from './pages/AdminServerVendorsPage';
import { AdminProtocolVersionsPage } from './pages/AdminProtocolVersionsPage';
import { AdminDriveManufacturersPage } from './pages/AdminDriveManufacturersPage';
import { AdminPumpVendorsPage } from './pages/AdminPumpVendorsPage';
import { AdminRmsManufacturersPage } from './pages/AdminRmsManufacturersPage';
import { AdminUserGroupsPage } from './pages/AdminUserGroupsPage';
import { TelemetryMonitorPage } from './pages/TelemetryMonitorPage';
import { SimulatorPage } from './pages/SimulatorPage';
import { AdminVfdModelsPage } from './pages/AdminVfdModelsPage';
import { AdminOrgsPage } from './pages/AdminOrgsPage';
import { AdminApiKeysPage } from './pages/AdminApiKeysPage';
import { AdminAuditPage } from './pages/AdminAuditPage';
import { AdminSchedulerPage } from './pages/AdminSchedulerPage';
import { AdminUsersPage } from './pages/AdminUsersPage';
import { AdminDnaPage } from './pages/AdminDnaPage';
import { AdminDeviceProfilesPage } from './pages/AdminDeviceProfilesPage';
import { AdminSimulatorSessionsPage } from './pages/AdminSimulatorSessionsPage';
import { AdminVfdCatalogOpsPage } from './pages/AdminVfdCatalogOpsPage';
import { LoginPage } from './pages/LoginPage';
import { InstallationsPage } from './pages/InstallationsPage';
import { DeviceInventoryPage } from './pages/DeviceInventoryPage';
import { DeviceDetailPage } from './pages/DeviceDetailPage';
import { TelemetryV2Page } from './pages/TelemetryV2Page';
import { CommandCenterPage } from './pages/CommandCenterPage';
import { RulesAlertsPage } from './pages/RulesAlertsPage';
import { AutomationPage } from './pages/AutomationPage';
import { TelemetryExportPage } from './pages/TelemetryExportPage';
import { CommandCatalogAdminPage } from './pages/CommandCatalogAdminPage';
import { ReportsPage } from './pages/ReportsPage';

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="login" element={<LoginPage />} />
        <Route path="simulator" element={<SimulatorPage />} />
        <Route element={<RequireAuth />}>
          <Route index element={<DashboardPage />} />
          <Route path="devices/enroll" element={<DeviceEnrollmentPage />} />
          <Route
            path="devices/configuration"
            element={<Navigate to="/devices/configuration/internal" replace />}
          />
          <Route
            path="devices/configuration/internal"
            element={<DeviceInternalCredentialsPage />}
          />
          <Route
            path="devices/configuration/government"
            element={<DeviceGovernmentCredentialsPage />}
          />
          <Route path="devices/configuration/drive" element={<DeviceDriveConfigPage />} />
          <Route path="devices/import" element={<DeviceImportPage />} />
          <Route path="devices/import/jobs" element={<DeviceImportJobsPage />} />
          <Route path="telemetry" element={<TelemetryMonitorPage />} />
          <Route path="telemetry/v2" element={<TelemetryV2Page />} />
          <Route path="telemetry/export" element={<TelemetryExportPage />} />
          <Route path="operations/installations" element={<InstallationsPage />} />
          <Route
            path="admin/installations"
            element={<Navigate to="/operations/installations" replace />}
          />
          <Route path="operations/command-center" element={<CommandCenterPage />} />
          <Route path="operations/command-catalog" element={<CommandCatalogAdminPage />} />
          <Route path="operations/rules-alerts" element={<RulesAlertsPage />} />
          <Route path="operations/automation" element={<AutomationPage />} />
          <Route path="live/device-inventory" element={<DeviceInventoryPage />} />
          <Route path="live/device-inventory/:idOrUuid" element={<DeviceDetailPage />} />
          <Route path="reports" element={<ReportsPage />} />
          <Route path="admin/states" element={<AdminStatesPage />} />
          <Route path="admin/state-authorities" element={<AdminStateAuthoritiesPage />} />
          <Route path="admin/projects" element={<AdminProjectsPage />} />
          <Route path="admin/orgs" element={<AdminOrgsPage />} />
          <Route path="admin/apikeys" element={<AdminApiKeysPage />} />
          <Route path="admin/audit" element={<AdminAuditPage />} />
          <Route path="admin/scheduler" element={<AdminSchedulerPage />} />
          <Route path="admin/dna" element={<AdminDnaPage />} />
          <Route path="admin/device-profiles" element={<AdminDeviceProfilesPage />} />
          <Route path="admin/simulator-sessions" element={<AdminSimulatorSessionsPage />} />
          <Route path="admin/server-vendors" element={<AdminServerVendorsPage />} />
          <Route path="admin/users" element={<AdminUsersPage />} />
          <Route path="admin/user-groups" element={<AdminUserGroupsPage />} />
          <Route path="admin/protocol-versions" element={<AdminProtocolVersionsPage />} />
          <Route path="admin/drive-manufacturers" element={<AdminDriveManufacturersPage />} />
          <Route path="admin/pump-vendors" element={<AdminPumpVendorsPage />} />
          <Route path="admin/rms-manufacturers" element={<AdminRmsManufacturersPage />} />
          <Route path="admin/vfd-models" element={<AdminVfdModelsPage />} />
          <Route path="admin/vfd-catalog-ops" element={<AdminVfdCatalogOpsPage />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/login" replace />} />
    </Routes>
  );
}
