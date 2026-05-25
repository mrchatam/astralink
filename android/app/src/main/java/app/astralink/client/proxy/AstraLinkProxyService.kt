package app.astralink.client.proxy

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.IBinder
import app.astralink.client.ProfileLinks
import app.astralink.client.core.AstraLinkProcessManager
import app.astralink.client.model.ConnectionProfile
import java.io.File

class AstraLinkProxyService : Service() {
    private lateinit var processManager: AstraLinkProcessManager

    override fun onCreate() {
        super.onCreate()
        processManager = AstraLinkProcessManager(this)
        ensureChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val profile = intent?.getStringExtra(EXTRA_PROFILE_JSON)?.let { ProfileLinks.import(it) }
            ?: return START_NOT_STICKY
        val config = File(cacheDir, "astralink_client.toml")
        val resolvers = File(cacheDir, "astralink_resolvers.txt")
        processManager.start(profile, config, resolvers)
        startForeground(NOTIFICATION_ID, buildNotification(profile))
        return START_STICKY
    }

    override fun onDestroy() {
        processManager.stop()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun ensureChannel() {
        val mgr = getSystemService(NotificationManager::class.java)
        mgr.createNotificationChannel(
            NotificationChannel(CHANNEL_ID, "AstraLink Proxy", NotificationManager.IMPORTANCE_LOW),
        )
    }

    private fun buildNotification(profile: ConnectionProfile): Notification {
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("AstraLink proxy")
            .setContentText(profile.domain)
            .setSmallIcon(android.R.drawable.stat_sys_download_done)
            .build()
    }

    companion object {
        const val EXTRA_PROFILE_JSON = "profile_json"
        private const val CHANNEL_ID = "astralink_proxy"
        private const val NOTIFICATION_ID = 41
    }
}
