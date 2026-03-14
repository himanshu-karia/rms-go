package com.autogridmobility.rmsmqtt1.utils

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

class MobileSessionManager(context: Context) : MobileSessionStore {
    private val prefs: SharedPreferences = createPreferences(context)

    companion object {
        private const val PREF_NAME = "mobile_session"
        private const val KEY_ACCESS_TOKEN = "access_token"
        private const val KEY_REFRESH_TOKEN = "refresh_token"
        private const val KEY_PHONE = "phone"

        private fun createPreferences(context: Context): SharedPreferences {
            return try {
                val masterKey = MasterKey.Builder(context)
                    .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
                    .build()

                EncryptedSharedPreferences.create(
                    context,
                    PREF_NAME,
                    masterKey,
                    EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                    EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
                )
            } catch (_: Exception) {
                context.getSharedPreferences(PREF_NAME, Context.MODE_PRIVATE)
            }
        }
    }

    override fun saveSession(accessToken: String, refreshToken: String, phone: String) {
        prefs.edit()
            .putString(KEY_ACCESS_TOKEN, accessToken)
            .putString(KEY_REFRESH_TOKEN, refreshToken)
            .putString(KEY_PHONE, phone)
            .apply()
    }

    override fun clearSession() {
        prefs.edit().clear().apply()
    }

    override fun getAccessToken(): String = prefs.getString(KEY_ACCESS_TOKEN, "") ?: ""

    override fun getRefreshToken(): String = prefs.getString(KEY_REFRESH_TOKEN, "") ?: ""

    override fun getPhone(): String = prefs.getString(KEY_PHONE, "") ?: ""

    override fun hasSession(): Boolean = getAccessToken().isNotBlank()
}