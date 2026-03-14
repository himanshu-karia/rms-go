package com.autogridmobility.rmsmqtt1.ui.navigation

sealed class Screen(val route: String) {
    object Login : Screen("login")
    object Home : Screen("home")
    object UxDashboard : Screen("ux_dashboard")
    object Dashboard : Screen("dashboard")
    object RawData : Screen("raw_data")
    object Settings : Screen("settings")
    object Graphs : Screen("graphs")
    object AdminCommandCatalog : Screen("admin_command_catalog")
    object AdminSimulatorSessions : Screen("admin_simulator_sessions")
    object AdminProjects : Screen("admin_projects")
    object AdminUsers : Screen("admin_users")
    object AdminApiKeys : Screen("admin_apikeys")
    object AdminHierarchy : Screen("admin_hierarchy")
    object AdminCatalogs : Screen("admin_catalogs")
    object AdminOrgs : Screen("admin_orgs")
    object AdminUserGroups : Screen("admin_user_groups")
}

enum class DashboardTab(val title: String) {
    HEARTBEAT("Heartbeat"),
    DATA("Data"),
    DAQ("DAQ"),
    ON_DEMAND("On Demand")
}

enum class RawDataTab(val title: String) {
    HEARTBEAT("Heartbeat"),
    DATA("Data"),
    DAQ("DAQ")
}
