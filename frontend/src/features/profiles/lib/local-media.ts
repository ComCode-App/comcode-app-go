import { profilesService } from "@/features/profiles/services/profiles.service";

/**
 * Browser-local media (an unsaved file picker preview or a vCard photo data URL).
 * The API only accepts media it stored itself, so these are uploaded after save.
 */
export function isLocalMediaUrl(url?: string | null): url is string {
  return !!url && (url.startsWith("blob:") || url.startsWith("data:"));
}

async function localMediaToFile(url: string, basename: string): Promise<File> {
  const blob = await (await fetch(url)).blob();
  const ext = blob.type.split("/")[1]?.split("+")[0] || "jpg";
  return new File([blob], `${basename}.${ext}`, {
    type: blob.type || "image/jpeg",
  });
}

/** Splits form media into API-owned URLs for the payload and local media to upload afterwards. */
export function splitProfileMedia(values: {
  avatar_url?: string | null;
  cover_url?: string | null;
}) {
  const localAvatar = isLocalMediaUrl(values.avatar_url)
    ? values.avatar_url
    : null;
  const localCover = isLocalMediaUrl(values.cover_url)
    ? values.cover_url
    : null;
  return {
    avatar_url: localAvatar ? "" : values.avatar_url || "",
    cover_url: localCover ? "" : values.cover_url || "",
    async uploadPending(profileUuid: string) {
      if (localAvatar) {
        await profilesService.uploadAvatar(
          profileUuid,
          await localMediaToFile(localAvatar, "avatar"),
        );
      }
      if (localCover) {
        await profilesService.uploadCover(
          profileUuid,
          await localMediaToFile(localCover, "cover"),
        );
      }
    },
  };
}
