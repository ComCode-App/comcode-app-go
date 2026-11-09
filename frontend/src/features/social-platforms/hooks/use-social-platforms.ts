import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { appToast } from "@/providers/toast-provider";
import {
  socialPlatformsService,
  type UpsertSocialPlatformPayload,
} from "../services/social-platforms.service";
import { socialPlatformsKeys } from "./query-keys";

export function useActiveSocialPlatformsQuery() {
  return useQuery({
    queryKey: socialPlatformsKeys.active(),
    queryFn: () => socialPlatformsService.listActive(),
    staleTime: 5 * 60 * 1000,
  });
}

export function useSocialPlatformMutations() {
  const queryClient = useQueryClient();

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: socialPlatformsKeys.all });
  };

  const createMutation = useMutation({
    mutationFn: (body: UpsertSocialPlatformPayload) =>
      socialPlatformsService.create(body),
    onSuccess: () => {
      appToast.success("Platform successfully created");
      invalidate();
    },
    onError: (err: Error) => {
      appToast.error(err.message || "Failed to create platform");
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({
      uuid,
      body,
    }: {
      uuid: string;
      body: UpsertSocialPlatformPayload;
    }) => socialPlatformsService.update(uuid, body),
    onSuccess: () => {
      appToast.success("Platform successfully updated");
      invalidate();
    },
    onError: (err: Error) => {
      appToast.error(err.message || "Failed to update platform");
    },
  });

  const uploadIconMutation = useMutation({
    mutationFn: ({ uuid, file }: { uuid: string; file: File }) =>
      socialPlatformsService.uploadIcon(uuid, file),
    onSuccess: () => {
      appToast.success("Platform icon uploaded");
      invalidate();
    },
    onError: (err: Error) => {
      appToast.error(err.message || "Failed to upload icon");
    },
  });

  const fetchIconMutation = useMutation({
    mutationFn: (uuid: string) => socialPlatformsService.fetchIcon(uuid),
    onSuccess: () => {
      appToast.success("Icon fetched via OpenGraph");
      invalidate();
    },
    onError: (err: Error) => {
      appToast.error(err.message || "Failed to fetch icon");
    },
  });

  const bulkFetchIconsMutation = useMutation({
    mutationFn: (uuids?: string[]) =>
      socialPlatformsService.bulkFetchIcons(uuids),
    onSuccess: (data) => {
      appToast.success(`Updated icons for ${data.updated_count} platforms`);
      invalidate();
    },
    onError: (err: Error) => {
      appToast.error(err.message || "Failed to bulk fetch icons");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => socialPlatformsService.remove(uuid),
    onSuccess: () => {
      appToast.success("Platform deleted");
      invalidate();
    },
    onError: (err: Error) => {
      appToast.error(err.message || "Failed to delete platform");
    },
  });

  return {
    createMutation,
    updateMutation,
    uploadIconMutation,
    fetchIconMutation,
    bulkFetchIconsMutation,
    deleteMutation,
  };
}
