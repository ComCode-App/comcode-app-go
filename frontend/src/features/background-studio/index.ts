export { BackgroundStudioPage } from "./components/background-studio-page";
export { BackgroundViewer } from "./components/background-viewer";
export type { BackgroundDocument, BackgroundKind } from "./types";
export { backgroundStudioNavItem } from "./nav";
// Non-component helpers (parseBackgroundDocument, backgroundsService) are imported by path from
// client code: re-exporting them here would pull the shader registry into server components.
