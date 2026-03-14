package com.autogridmobility.rmsmqtt1.utils

interface MobileSessionStore {
    fun saveSession(accessToken: String, refreshToken: String, phone: String)
    fun clearSession()
    fun getAccessToken(): String
    fun getRefreshToken(): String
    fun getPhone(): String
    fun hasSession(): Boolean
}
