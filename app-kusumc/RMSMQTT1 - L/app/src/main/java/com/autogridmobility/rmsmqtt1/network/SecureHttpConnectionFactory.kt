package com.autogridmobility.rmsmqtt1.network

import java.net.HttpURLConnection
import java.net.URL
import java.security.KeyStore
import java.security.MessageDigest
import java.security.cert.X509Certificate
import java.util.Base64
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager

object SecureHttpConnectionFactory {
    private val isDebugMode: Boolean = false

    private val debugCleartextHosts = emptySet<String>()
    private val releaseAllowedHosts = emptySet<String>()
    private val pinnedSpkiByHost: Map<String, Set<String>> = emptyMap()

    fun open(urlString: String): HttpURLConnection {
        val url = URL(urlString)
        return when (url.protocol.lowercase()) {
            "https" -> {
                val connection = (url.openConnection() as HttpsURLConnection)
                applyHostnamePolicy(connection, url.host)
                applyPinning(connection, url.host)
                connection
            }
            "http" -> {
                if (!isCleartextAllowed(url.host)) {
                    throw SecurityException("Cleartext HTTP is not allowed for host ${url.host}")
                }
                url.openConnection() as HttpURLConnection
            }
            else -> throw SecurityException("Unsupported protocol ${url.protocol}")
        }
    }

    private fun isCleartextAllowed(host: String): Boolean {
        return isDebugMode && debugCleartextHosts.contains(host.lowercase())
    }

    private fun applyHostnamePolicy(connection: HttpsURLConnection, host: String) {
        val normalizedHost = host.lowercase()
        connection.hostnameVerifier = javax.net.ssl.HostnameVerifier { hostname, session ->
            val defaultOk = HttpsURLConnection.getDefaultHostnameVerifier().verify(hostname, session)
            if (!defaultOk) return@HostnameVerifier false

            if (isDebugMode) {
                return@HostnameVerifier true
            }

            if (releaseAllowedHosts.isEmpty()) {
                return@HostnameVerifier true
            }

            releaseAllowedHosts.contains(normalizedHost) && releaseAllowedHosts.contains(hostname.lowercase())
        }
    }

    private fun applyPinning(connection: HttpsURLConnection, host: String) {
        val configuredPins = pinnedSpkiByHost[host.lowercase()]?.filter { it.isNotBlank() }?.toSet().orEmpty()
        if (configuredPins.isEmpty()) {
            return
        }

        val defaultTrustManager = defaultX509TrustManager()
        val pinningManager = object : X509TrustManager {
            override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) {
                defaultTrustManager.checkClientTrusted(chain, authType)
            }

            override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) {
                defaultTrustManager.checkServerTrusted(chain, authType)
                val peerPins = chain.map { certificate -> certificate.publicKeyPin() }.toSet()
                if (configuredPins.intersect(peerPins).isEmpty()) {
                    throw SecurityException("Certificate pinning validation failed for host $host")
                }
            }

            override fun getAcceptedIssuers(): Array<X509Certificate> = defaultTrustManager.acceptedIssuers
        }

        val sslContext = SSLContext.getInstance("TLS")
        sslContext.init(null, arrayOf(pinningManager), null)
        connection.sslSocketFactory = sslContext.socketFactory
    }

    private fun defaultX509TrustManager(): X509TrustManager {
        val factory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
        factory.init(null as KeyStore?)
        val manager = factory.trustManagers.firstOrNull { it is X509TrustManager } as? X509TrustManager
        checkNotNull(manager) { "No default X509TrustManager available" }
        return manager
    }

    private fun X509Certificate.publicKeyPin(): String {
        val digest = MessageDigest.getInstance("SHA-256").digest(publicKey.encoded)
        val encoded = Base64.getEncoder().encodeToString(digest)
        return "sha256/$encoded"
    }
}
