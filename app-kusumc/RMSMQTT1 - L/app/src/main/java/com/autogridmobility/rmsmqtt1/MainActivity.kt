package com.autogridmobility.rmsmqtt1

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.clickable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Build
import androidx.compose.material.icons.filled.Dashboard
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Menu
import androidx.compose.material.icons.filled.Logout
import androidx.compose.material.icons.filled.ShowChart
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Star
import androidx.compose.material3.CenterAlignedTopAppBar
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalDrawerSheet
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.NavigationDrawerItem
import androidx.compose.material3.NavigationDrawerItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.material3.rememberDrawerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.autogridmobility.rmsmqtt1.ui.navigation.Screen
import com.autogridmobility.rmsmqtt1.ui.screens.DashboardScreen
import com.autogridmobility.rmsmqtt1.ui.screens.HomeScreen
import com.autogridmobility.rmsmqtt1.ui.screens.LoginScreen
import com.autogridmobility.rmsmqtt1.ui.screens.RawDataScreen
import com.autogridmobility.rmsmqtt1.ui.screens.SettingsScreenWithExport
import com.autogridmobility.rmsmqtt1.ui.screens.UxDashboardScreen
import com.autogridmobility.rmsmqtt1.ui.screens.GraphsScreen
import com.autogridmobility.rmsmqtt1.ui.screens.AdminCommandCatalogScreen
import com.autogridmobility.rmsmqtt1.ui.screens.AdminSimulatorSessionsScreen
import com.autogridmobility.rmsmqtt1.ui.screens.AdminProjectsScreen
import com.autogridmobility.rmsmqtt1.ui.screens.AdminUsersScreen
import com.autogridmobility.rmsmqtt1.ui.screens.AdminApiKeysScreen
import com.autogridmobility.rmsmqtt1.ui.screens.AdminHierarchyScreen
import com.autogridmobility.rmsmqtt1.ui.screens.AdminCatalogsScreen
import com.autogridmobility.rmsmqtt1.ui.screens.AdminOrgsScreen
import com.autogridmobility.rmsmqtt1.ui.screens.AdminUserGroupsScreen
import com.autogridmobility.rmsmqtt1.ui.theme.RMSMQTT1Theme
import com.autogridmobility.rmsmqtt1.viewmodel.MobileAuthViewModel
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            RMSMQTT1Theme {
                MainApp()
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainApp() {
    val authViewModel: MobileAuthViewModel = viewModel()

    val isAuthenticated by authViewModel.isAuthenticated.collectAsStateWithLifecycle()
    val isLoading by authViewModel.isLoading.collectAsStateWithLifecycle()
    val phone by authViewModel.phone.collectAsStateWithLifecycle()
    val otpRef by authViewModel.otpRef.collectAsStateWithLifecycle()
    val authError by authViewModel.error.collectAsStateWithLifecycle()
    val assignments by authViewModel.assignments.collectAsStateWithLifecycle()

    val navController = rememberNavController()
    val drawerState = rememberDrawerState(initialValue = DrawerValue.Closed)
    val scope = rememberCoroutineScope()
    var selectedRoute by remember { mutableStateOf(Screen.Home.route) }

    if (!isAuthenticated) {
        LoginScreen(
            phone = phone,
            otpRef = otpRef,
            isLoading = isLoading,
            error = authError,
            onPhoneChanged = authViewModel::updatePhone,
            onRequestOtp = {
                authViewModel.requestOtp(onRequested = {})
            },
            onVerifyOtp = { otp ->
                authViewModel.verifyOtp(otp, onSuccess = {
                    selectedRoute = Screen.Home.route
                })
            },
            onBypassLogin = {
                authViewModel.loginWithBypass(
                    username = "Him",
                    password = "0554",
                    onSuccess = { selectedRoute = Screen.Home.route }
                )
            },
            onClearError = authViewModel::clearError
        )
        return
    }

    ModalNavigationDrawer(
        drawerState = drawerState,
        drawerContent = {
            DrawerContent(
                selectedRoute = selectedRoute,
                onLogout = {
                    authViewModel.logout {
                        selectedRoute = Screen.Home.route
                    }
                },
                onNavigate = { route ->
                    selectedRoute = route
                    navController.navigate(route) {
                        // Clear the back stack and navigate to the selected screen
                        popUpTo(navController.graph.startDestinationId) {
                            saveState = true
                        }
                        launchSingleTop = true
                        restoreState = true
                    }
                    scope.launch {
                        drawerState.close()
                    }
                }
            )
        }
    ) {
        Scaffold(
            topBar = {
                CenterAlignedTopAppBar(
                    title = {
                        Text(
                            text = "PMKUSUM IoT Monitor",
                            style = MaterialTheme.typography.titleLarge,
                            fontWeight = FontWeight.Medium
                        )
                    },
                    navigationIcon = {
                        IconButton(
                            onClick = {
                                scope.launch {
                                    drawerState.open()
                                }
                            }
                        ) {
                            Icon(
                                imageVector = Icons.Default.Menu,
                                contentDescription = "Menu"
                            )
                        }
                    },
                    colors = TopAppBarDefaults.centerAlignedTopAppBarColors(
                        containerColor = MaterialTheme.colorScheme.surface,
                        titleContentColor = MaterialTheme.colorScheme.onSurface
                    )
                )
            }
        ) { paddingValues ->
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(paddingValues)
            ) {
                NavHost(
                    navController = navController,
                    startDestination = Screen.Home.route
                ) {
                    composable(Screen.Home.route) { HomeScreen(assignments = assignments) }
                    composable(Screen.UxDashboard.route) { UxDashboardScreen() }
                    composable(Screen.Dashboard.route) { DashboardScreen() }
                    composable(Screen.RawData.route) { RawDataScreen() }
                    composable(Screen.Settings.route) { SettingsScreenWithExport() }
                    composable(Screen.Graphs.route) { GraphsScreen() }
                    composable(Screen.AdminCommandCatalog.route) { AdminCommandCatalogScreen() }
                    composable(Screen.AdminSimulatorSessions.route) { AdminSimulatorSessionsScreen() }
                    composable(Screen.AdminProjects.route) { AdminProjectsScreen() }
                    composable(Screen.AdminUsers.route) { AdminUsersScreen() }
                    composable(Screen.AdminApiKeys.route) { AdminApiKeysScreen() }
                    composable(Screen.AdminHierarchy.route) { AdminHierarchyScreen() }
                    composable(Screen.AdminCatalogs.route) { AdminCatalogsScreen() }
                    composable(Screen.AdminOrgs.route) { AdminOrgsScreen() }
                    composable(Screen.AdminUserGroups.route) { AdminUserGroupsScreen() }
                }
            }
        }
    }
}

@Composable
fun DrawerContent(
    selectedRoute: String,
    onLogout: () -> Unit,
    onNavigate: (String) -> Unit
) {
    val standaloneItems = listOf(
        DrawerItem("RMS Dashboard", Screen.Home.route, Icons.Default.Home)
    )

    val groupedItems = listOf(
        DrawerSection(
            id = "live",
            title = "Live",
            items = listOf(
                DrawerItem("UX Dashboard", Screen.UxDashboard.route, Icons.Default.Dashboard),
                DrawerItem("Dashboard", Screen.Dashboard.route, Icons.Default.Star),
                DrawerItem("Graphs", Screen.Graphs.route, Icons.Default.ShowChart),
                DrawerItem("Raw Data", Screen.RawData.route, Icons.Default.Build),
            )
        ),
        DrawerSection(
            id = "operations",
            title = "Operations",
            items = listOf(
                DrawerItem("Settings", Screen.Settings.route, Icons.Default.Settings),
                DrawerItem("Command Catalog", Screen.AdminCommandCatalog.route, Icons.Default.Build),
            )
        ),
        DrawerSection(
            id = "administration",
            title = "Administration",
            items = listOf(
                DrawerItem("Projects", Screen.AdminProjects.route, Icons.Default.Star),
                DrawerItem("Users", Screen.AdminUsers.route, Icons.Default.Home),
                DrawerItem("API Keys", Screen.AdminApiKeys.route, Icons.Default.Settings),
                DrawerItem("Organizations", Screen.AdminOrgs.route, Icons.Default.Home),
                DrawerItem("User Groups", Screen.AdminUserGroups.route, Icons.Default.Star),
                DrawerItem("Hierarchy", Screen.AdminHierarchy.route, Icons.Default.Dashboard),
                DrawerItem("Catalogs", Screen.AdminCatalogs.route, Icons.Default.Build),
                DrawerItem("Simulator Sessions", Screen.AdminSimulatorSessions.route, Icons.Default.Dashboard),
            )
        )
    )

    val activeSectionId = groupedItems.firstOrNull { section ->
        section.items.any { it.route == selectedRoute }
    }?.id

    var openSectionId by remember { mutableStateOf<String?>(null) }
    val resolvedOpenSectionId = openSectionId ?: activeSectionId

    ModalDrawerSheet(
        modifier = Modifier.fillMaxWidth(0.75f),
        drawerShape = RoundedCornerShape(topEnd = 16.dp, bottomEnd = 16.dp)
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
        ) {
            // Drawer Header
            Surface(
                modifier = Modifier.fillMaxWidth(),
                color = MaterialTheme.colorScheme.primary,
                shape = RoundedCornerShape(bottomEnd = 16.dp)
            ) {
                Column(
                    modifier = Modifier.padding(24.dp)
                ) {
                    Text(
                        text = "PMKUSUM IoT",
                        style = MaterialTheme.typography.headlineSmall,
                        color = MaterialTheme.colorScheme.onPrimary,
                        fontWeight = FontWeight.Bold
                    )
                    Text(
                        text = "Version 1.0.0 (Demo)",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f)
                    )
                }
            }
            
            Spacer(modifier = Modifier.height(16.dp))

            standaloneItems.forEach { item ->
                NavigationDrawerItem(
                    icon = {
                        Icon(
                            imageVector = item.icon,
                            contentDescription = item.title
                        )
                    },
                    label = {
                        Text(
                            text = item.title,
                            style = MaterialTheme.typography.labelLarge
                        )
                    },
                    selected = selectedRoute == item.route,
                    onClick = {
                        if (item.route == "logout") {
                            onLogout()
                        } else {
                            onNavigate(item.route)
                        }
                    },
                    modifier = Modifier.padding(horizontal = 12.dp),
                    colors = NavigationDrawerItemDefaults.colors(
                        selectedContainerColor = MaterialTheme.colorScheme.primary.copy(alpha = 0.1f),
                        selectedIconColor = MaterialTheme.colorScheme.primary,
                        selectedTextColor = MaterialTheme.colorScheme.primary
                    )
                )
            }

            Spacer(modifier = Modifier.height(8.dp))

            groupedItems.forEach { section ->
                val isOpen = resolvedOpenSectionId == section.id
                val hasActiveChild = section.items.any { it.route == selectedRoute }

                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable {
                            openSectionId = if (openSectionId == section.id) null else section.id
                        }
                        .padding(horizontal = 16.dp, vertical = 10.dp),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text(
                        text = section.title,
                        style = MaterialTheme.typography.titleSmall,
                        color = if (hasActiveChild) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                        fontWeight = FontWeight.SemiBold
                    )
                    Icon(
                        imageVector = if (isOpen) Icons.Default.KeyboardArrowDown else Icons.Default.KeyboardArrowRight,
                        contentDescription = if (isOpen) "Collapse ${section.title}" else "Expand ${section.title}",
                        tint = if (hasActiveChild) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }

                if (isOpen) {
                    section.items.forEach { item ->
                        NavigationDrawerItem(
                            icon = {
                                Icon(
                                    imageVector = item.icon,
                                    contentDescription = item.title
                                )
                            },
                            label = {
                                Text(
                                    text = item.title,
                                    style = MaterialTheme.typography.labelLarge
                                )
                            },
                            selected = selectedRoute == item.route,
                            onClick = { onNavigate(item.route) },
                            modifier = Modifier.padding(start = 24.dp, end = 12.dp),
                            colors = NavigationDrawerItemDefaults.colors(
                                selectedContainerColor = MaterialTheme.colorScheme.primary.copy(alpha = 0.1f),
                                selectedIconColor = MaterialTheme.colorScheme.primary,
                                selectedTextColor = MaterialTheme.colorScheme.primary
                            )
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(12.dp))

            NavigationDrawerItem(
                icon = {
                    Icon(
                        imageVector = Icons.Default.Logout,
                        contentDescription = "Logout"
                    )
                },
                label = {
                    Text(
                        text = "Logout",
                        style = MaterialTheme.typography.labelLarge
                    )
                },
                selected = false,
                onClick = onLogout,
                modifier = Modifier.padding(horizontal = 12.dp),
            )
        }
    }
}

data class DrawerItem(
    val title: String,
    val route: String,
    val icon: ImageVector
)

data class DrawerSection(
    val id: String,
    val title: String,
    val items: List<DrawerItem>
)