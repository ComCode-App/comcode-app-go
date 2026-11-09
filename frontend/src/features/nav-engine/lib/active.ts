export function isNavHrefActive(
  pathname: string,
  href: string,
  homeHref: string,
  exact = false,
): boolean {
  return (
    pathname === href ||
    (!exact && href !== homeHref && pathname.startsWith(`${href}/`))
  );
}

export function isNavItemActive(
  pathname: string,
  item: {
    href?: string;
    exact?: boolean;
    items?: Array<{ href?: string; exact?: boolean }>;
  },
  homeHref: string,
): boolean {
  if (item.items?.length) {
    return item.items.some(
      (child) =>
        child.href &&
        isNavHrefActive(pathname, child.href, homeHref, child.exact),
    );
  }
  return item.href
    ? isNavHrefActive(pathname, item.href, homeHref, item.exact)
    : false;
}

export function formatNavCount(value: number, max = 99): string {
  if (value > max) return `${max}+`;
  return String(value);
}
