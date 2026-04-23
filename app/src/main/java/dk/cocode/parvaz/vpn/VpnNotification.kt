package dk.cocode.parvaz.vpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.app.NotificationCompat
import dk.cocode.parvaz.MainActivity
import dk.cocode.parvaz.R

/**
 * Ongoing notification shown while ParvazVpnService holds the VPN.
 * Required for foreground-service promotion on API 26+ (below that we'd
 * skip the notification, but minSdk is 24 so Android N still shows it).
 *
 * Tapping reopens MainActivity; the action button stops the VPN from the
 * notification tray so the user doesn't have to come back into the app.
 */
object VpnNotification {
    const val CHANNEL_ID = "parvaz-vpn-ongoing"

    fun build(context: Context, ongoing: Boolean): Notification {
        ensureChannel(context)

        val openApp = PendingIntent.getActivity(
            context,
            0,
            Intent(context, MainActivity::class.java)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val stop = PendingIntent.getService(
            context,
            1,
            Intent(context, ParvazVpnService::class.java).setAction(ParvazVpnService.ACTION_STOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )

        return NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle(context.getString(R.string.vpn_notification_title))
            .setContentText(context.getString(R.string.vpn_notification_text))
            .setContentIntent(openApp)
            .addAction(
                0,
                context.getString(R.string.vpn_notification_stop_action),
                stop,
            )
            .setOngoing(ongoing)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .build()
    }

    private fun ensureChannel(context: Context) {
        // Notification channels are API 26+. On API 24-25 the notification
        // builder just uses the legacy priority/category and there's no
        // channel to register.
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val manager = context.getSystemService(NotificationManager::class.java) ?: return
        if (manager.getNotificationChannel(CHANNEL_ID) != null) return
        val channel = NotificationChannel(
            CHANNEL_ID,
            context.getString(R.string.vpn_notification_channel_name),
            NotificationManager.IMPORTANCE_LOW,
        ).apply {
            description = context.getString(R.string.vpn_notification_channel_description)
            setShowBadge(false)
        }
        manager.createNotificationChannel(channel)
    }
}
