# Android Project Build Template - Issue Prevention Prompt

## 🎯 **Critical Instructions for Clean Android Project Setup**

### **ARCHITECTURE REQUIREMENTS:**
- **Use ONLY Jetpack Compose** - No XML layouts, fragments, or View-based components
- **Single Activity Architecture** with Compose Navigation only
- **MVVM pattern** with ViewModels and StateFlow/Compose State
- **Remove ALL generated template files** immediately after project creation

### **PROJECT TEMPLATE CLEANUP (MANDATORY):**
```
DELETE these files immediately after project creation:
- app/src/main/res/layout/*.xml (ALL layout files)
- app/src/main/res/menu/*.xml (ALL menu files) 
- app/src/main/res/navigation/*.xml (XML navigation files)
- app/src/main/java/**/ui/home/HomeFragment.kt
- app/src/main/java/**/ui/gallery/GalleryFragment.kt
- app/src/main/java/**/ui/slideshow/SlideshowFragment.kt
- Any other Fragment classes from templates
```

### **GRADLE CONFIGURATION (CRITICAL):**

#### **1. Dependencies - Use LATEST STABLE versions:**
```kotlin
// In libs.versions.toml - UPDATE these version numbers
kotlin = "2.0.21"  // Latest stable
composeBom = "2024.12.01"  // Latest Compose BOM
composeActivity = "1.9.3"
composeNavigation = "2.8.5"
```

#### **2. MANDATORY Packaging Block (Prevents META-INF conflicts):**
```kotlin
// In app/build.gradle.kts - ADD THIS BLOCK
android {
    packaging {
        resources {
            excludes += "META-INF/INDEX.LIST"
            excludes += "META-INF/DEPENDENCIES"
            excludes += "META-INF/LICENSE"
            excludes += "META-INF/LICENSE.txt"
            excludes += "META-INF/NOTICE"
            excludes += "META-INF/NOTICE.txt"
            excludes += "META-INF/io.netty.versions.properties"
            excludes += "META-INF/*.kotlin_module"
        }
    }
}
```

#### **3. CompileSdk Warning Suppression:**
```properties
# In gradle.properties - ADD THIS LINE
android.suppressUnsupportedCompileSdk=36
```

### **IMPORT STATEMENTS (PREVENT MISSING IMPORTS):**
Always include these essential Compose imports in UI files:
```kotlin
import androidx.compose.ui.Alignment  // CRITICAL - often missing
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.navigation.compose.*
```

### **TYPE SAFETY (PREVENT COMPILATION ERRORS):**
- **Always use explicit Double types** in Random.nextDouble(): `Random.nextDouble(0.0, 100.0)` NOT `Random.nextDouble(0, 100)`
- **Use proper type annotations** for data classes with serialization
- **Avoid Int/Double type mismatches** in data generation and calculations

### **NETWORK LIBRARY HANDLING:**
For projects using networking libraries (MQTT, HTTP clients):
- **Prefer OkHttp/Retrofit** over io.netty-based libraries when possible
- **If using HiveMQ MQTT or similar netty-based libraries:**
  - ALWAYS add the packaging exclusions above
  - Test build immediately after adding the dependency
  - Add exclusions preemptively, don't wait for conflicts

### **MANIFEST CONFIGURATION:**
- **Use ONLY required permissions** - avoid deprecated ones
- **Remove unused theme references** from AndroidManifest.xml
- **Clean up theme files** - remove unused AppBarOverlay, PopupOverlay styles for Compose-only apps

### **MAINACTIVITY PATTERN:**
```kotlin
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            YourAppTheme {
                // ONLY Compose content here
                // NO setContentView()
                // NO findViewById()
                // NO Fragment transactions
            }
        }
    }
}
```

### **BUILD TESTING STRATEGY:**
1. **Clean build immediately** after dependency additions
2. **Test compilation** before adding complex logic
3. **Run `:app:assembleDebug`** to catch resource conflicts early
4. **Never ignore packaging warnings** - fix them immediately

### **THEME CONFIGURATION:**
- **Use Material3 themes only** for Compose
- **Remove View-based theme styles** (AppBarOverlay, PopupOverlay)
- **Ensure consistent dark theme** across all Compose screens

### **DATA CLASS BEST PRACTICES:**
```kotlin
@Serializable
data class YourData(
    val numericField: Double,  // Use Double for calculations
    val stringField: String,
    val intField: Int = 0      // Default values prevent issues
)
```

### **AVOID THESE COMMON PITFALLS:**
❌ **DON'T**: Mix Compose with XML layouts  
❌ **DON'T**: Use Fragment-based navigation with Compose  
❌ **DON'T**: Ignore packaging conflicts (META-INF errors)  
❌ **DON'T**: Use outdated Compose BOM versions  
❌ **DON'T**: Skip import statement verification  
❌ **DON'T**: Use implicit Int types where Double expected  

✅ **DO**: Pure Compose architecture  
✅ **DO**: Add packaging exclusions preemptively  
✅ **DO**: Use latest stable dependency versions  
✅ **DO**: Clean template files immediately  
✅ **DO**: Test build after each major dependency addition  

### **VERIFICATION CHECKLIST:**
Before proceeding with feature development:
- [ ] All template XML files deleted
- [ ] All Fragment classes removed  
- [ ] Packaging block added to build.gradle.kts
- [ ] Latest Compose BOM version used
- [ ] CompileSdk warning suppressed
- [ ] Clean build passes without errors
- [ ] Only Compose imports used in UI files
- [ ] MainActivity uses setContent() only

### **PROJECT STRUCTURE TEMPLATE:**
```
app/src/main/java/com/yourpackage/yourapp/
├── MainActivity.kt (Compose only)
├── data/ (data classes with @Serializable)
├── ui/
│   ├── components/ (reusable Compose components)
│   ├── screens/ (Compose screens)
│   ├── navigation/ (Compose navigation)
│   └── theme/ (Material3 theme)
├── viewmodel/ (ViewModels with StateFlow)
├── service/ (background services if needed)
└── utils/ (utility classes)
```

**USE THIS TEMPLATE**: Copy and adapt this prompt for any Android project to avoid the specific build issues, dependency conflicts, and architectural problems we encountered in the PMKUSUM IoT app.
