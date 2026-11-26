/**
 * Profile cover image switch (TEC-1305). The cover image is hidden across the app: the
 * profile editor, the live card preview, the profile detail page, public profile pages
 * and the route QR logo picker. Stored `cover_url` values and the API stay as they are,
 * so setting this back to `true` restores the feature everywhere.
 */
export const PROFILE_COVER_ENABLED = false;
