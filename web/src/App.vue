<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, provide, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import logoDarkImg from "./assets/glipz-dark.png";
import logoLightImg from "./assets/glipz-light.png";
import Icon from "./components/Icon.vue";
import SidebarComposeModal from "./components/SidebarComposeModal.vue";
import SidebarWidgetHost from "./components/SidebarWidgetHost.vue";
import UserBadges from "./components/UserBadges.vue";
import { ACCESS, clearTokens, getAccessToken } from "./auth";
import { api, displayInstanceDomain } from "./lib/api";
import { displayName as displayNameFromEmail } from "./lib/feedDisplay";
import {
  applyTheme,
  persistThemeModePreference,
  persistThemePreference,
  readStoredThemeModePreference,
  readStoredThemePreference,
  systemThemeMediaQuery,
  type ResolvedTheme,
  type ThemeModePreference,
  type ThemePreference,
} from "./lib/theme";
import { connectNotifyStream, notifyToastMessage, type NotifyPayload } from "./lib/notifyStream";
import { connectDMStream, type DmStreamPayload } from "./lib/dmStream";
import { clearRememberedUnlockedIdentity } from "./lib/dmUnlockMemory";
import { meHubTick } from "./meHub";
import {
  incrementUnreadNotificationCount,
  pingNotificationReceived,
  refreshUnreadNotificationCount,
  unreadNotificationCount,
} from "./notificationHub";
import {
  incrementUnreadDMCount,
  pingDMReceived,
  refreshUnreadDMCount,
  unreadDMCount,
} from "./dmHub";
import { getOperatorAnnouncements } from "./data/operatorAnnouncements";
import { fetchPublicInstanceSettings, type OperatorAnnouncement } from "./lib/instanceSettings";
import { legalDocumentLink, type LegalDocumentKey, type LegalDocumentURLSettings } from "./lib/legalDocumentLinks";
import { safeHttpURL } from "./lib/redirect";
import { APP_VERSION } from "./lib/appInfo";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const authed = ref(!!getAccessToken());
const me = ref<{
  id: string;
  email: string;
  handle: string;
  display_name: string;
  badges?: string[];
  avatar_url: string | null;
  is_site_admin?: boolean;
  fanclub_patreon_enabled?: boolean;
} | null>(null);
provide("appMe", me);
/** Falls back to initials when loading an avatar URL fails. */
const avatarImgFailed = ref(false);
const profileMenuOpen = ref(false);
const profileMenuRoot = ref<HTMLElement | null>(null);
const sidebarComposeOpen = ref(false);
/** Shows the sidebar as a drawer below the md breakpoint. */
const mobileNavOpen = ref(false);
const appHeaderEl = ref<HTMLElement | null>(null);
const appHeaderOffset = ref("56px");
const viewportHeight = ref("100dvh");
const searchQuery = ref("");
const themePreference = ref<ThemePreference>(readStoredThemePreference());
const themeModePreference = ref<ThemeModePreference>(readStoredThemeModePreference());
const resolvedTheme = ref<ResolvedTheme>("light");
const logoImg = computed(() => (resolvedTheme.value === "dark" ? logoDarkImg : logoLightImg));
provide("resolvedTheme", resolvedTheme);
const sidebarWidgetViewer = computed(() =>
  me.value
    ? {
        id: me.value.id,
        email: me.value.email,
        handle: me.value.handle,
        display_name: me.value.display_name,
        avatar_url: me.value.avatar_url,
        is_site_admin: me.value.is_site_admin,
        fanclub_patreon_enabled: me.value.fanclub_patreon_enabled,
      }
    : null,
);

function setThemePreferenceSilent(next: ThemePreference) {
  themePreference.value = next;
}
provide("setThemePreferenceSilent", setThemePreferenceSilent);

function setThemeModePreferenceSilent(next: ThemeModePreference) {
  themeModePreference.value = next;
}
provide("setThemeModePreferenceSilent", setThemeModePreferenceSilent);

const operatorAnnouncements = ref<OperatorAnnouncement[]>(getOperatorAnnouncements());
const operatorAnnouncementIndex = ref(0);
const legalDocumentUrls = ref<LegalDocumentURLSettings>({});
let operatorAnnouncementTimer: ReturnType<typeof setInterval> | null = null;

const currentOperatorAnnouncement = computed(() => operatorAnnouncements.value[operatorAnnouncementIndex.value] ?? null);
const hasMultipleOperatorAnnouncements = computed(() => operatorAnnouncements.value.length > 1);

async function loadOperatorAnnouncements() {
  try {
    const settings = await fetchPublicInstanceSettings();
    legalDocumentUrls.value = settings;
    operatorAnnouncements.value = settings.operator_announcements.length
      ? settings.operator_announcements
      : getOperatorAnnouncements();
  } catch {
    operatorAnnouncements.value = getOperatorAnnouncements();
  }
}

function clampOperatorAnnouncementIndex() {
  const len = operatorAnnouncements.value.length;
  if (len <= 0) {
    operatorAnnouncementIndex.value = 0;
    return;
  }
  if (operatorAnnouncementIndex.value >= len) {
    operatorAnnouncementIndex.value = 0;
  }
}

function showOperatorAnnouncement(index: number) {
  const len = operatorAnnouncements.value.length;
  if (len <= 0) return;
  operatorAnnouncementIndex.value = (index + len) % len;
}

function showPreviousOperatorAnnouncement() {
  showOperatorAnnouncement(operatorAnnouncementIndex.value - 1);
}

function showNextOperatorAnnouncement() {
  showOperatorAnnouncement(operatorAnnouncementIndex.value + 1);
}

function stopOperatorAnnouncementSlider() {
  if (operatorAnnouncementTimer) clearInterval(operatorAnnouncementTimer);
  operatorAnnouncementTimer = null;
}

function startOperatorAnnouncementSlider() {
  stopOperatorAnnouncementSlider();
  if (!hasMultipleOperatorAnnouncements.value) return;
  operatorAnnouncementTimer = setInterval(showNextOperatorAnnouncement, 7000);
}

function legalLink(key: LegalDocumentKey): { href: string; external: boolean } {
  return legalDocumentLink(legalDocumentUrls.value, key);
}

function currentDomain(): string {
  return displayInstanceDomain();
}

function looksLikeRemoteAcct(q: string): boolean {
  const s = q.trim().replace(/^@/, "");
  if (!s.includes("@")) return false;
  const parts = s.split("@");
  return parts.length === 2 && parts[0].length > 0 && parts[1].length > 0;
}

function looksLikeActorURL(q: string): boolean {
  return /^https?:\/\//i.test(q.trim());
}

function navigateFromSearch() {
  const q = searchQuery.value.trim();
  if (!q) return;
  if (looksLikeActorURL(q)) {
    router.push({ path: "/remote/profile", query: { actor: q.trim() } });
    searchQuery.value = "";
    return;
  }
  if (looksLikeRemoteAcct(q)) {
    const acct = q.trim().replace(/^@/, "");
    const at = acct.indexOf("@");
    const user = acct.slice(0, at);
    const host = acct.slice(at + 1);
    router.push({ path: `/@${user}@${host}` });
    searchQuery.value = "";
    return;
  }
  if (authed.value) {
    router.push({ path: "/search", query: { q } });
    searchQuery.value = "";
  }
}

function onGlobalSearchEnter() {
  navigateFromSearch();
}

function syncSearchQueryFromRoute() {
  if (route.path !== "/search") {
    searchQuery.value = "";
    return;
  }
  const raw = route.query.q;
  if (typeof raw === "string") {
    searchQuery.value = raw;
    return;
  }
  if (Array.isArray(raw)) {
    searchQuery.value = String(raw[0] ?? "");
    return;
  }
  searchQuery.value = "";
}

function syncAppHeaderOffset() {
  if (typeof window === "undefined") return;
  const height = appHeaderEl.value?.getBoundingClientRect().height ?? 56;
  appHeaderOffset.value = `${Math.max(56, Math.round(height))}px`;
  viewportHeight.value = `${window.visualViewport?.height ?? window.innerHeight}px`;
  if (window.innerWidth >= 768) mobileNavOpen.value = false;
}

let disconnectNotifyStream: (() => void) | null = null;
let disconnectDMStream: (() => void) | null = null;
const notifyToastMessageText = ref("");
let notifyToastTimer: ReturnType<typeof setTimeout> | null = null;
let themeMediaQuery: MediaQueryList | null = null;

function syncTheme() {
  resolvedTheme.value = applyTheme(themePreference.value, themeModePreference.value);
}

function onSystemThemeChange() {
  if (themeModePreference.value === "system") syncTheme();
}

const guestSimpleLayoutPaths = new Set([
  "/login",
  "/register",
  "/register/verify",
  "/mfa",
]);

function currentBrowserPath(): string {
  if (typeof window === "undefined") return "";
  return window.location.pathname.replace(/\/+$/, "") || "/";
}

function isGuestSimpleLayoutPath(path: string): boolean {
  const normalized = path.replace(/\/+$/, "") || "/";
  return guestSimpleLayoutPaths.has(normalized);
}

const usesGuestSimpleLayout = computed(() => {
  if (route.matched.some((record) => record.meta.guestSimpleLayout === true)) return true;
  if (isGuestSimpleLayoutPath(route.path) || isGuestSimpleLayoutPath(currentBrowserPath())) return true;
  if (route.path === "/about") return true;
  if (authed.value) return false;
  return (
    route.path === "/legal/terms"
    || route.path === "/legal/privacy"
    || route.path === "/legal/nsfw-guidelines"
    || route.path === "/legal/law-enforcement"
    || route.path === "/legal/api-guidelines"
    || route.path === "/federation/guidelines"
  );
});
const hideRightAside = computed(() => route.meta.hideRightAside === true);
const wideMain = computed(() => route.meta.wideMain === true);
const mobileEdgeToEdge = computed(() => route.meta.mobileEdgeToEdge === true);
const hideAppHeader = computed(() => route.meta.hideAppHeader === true);
const hideMobileChrome = computed(() => route.meta.hideMobileChrome === true);
const isAdminShell = computed(() => route.meta.adminShell === true);
const containedMainScroll = computed(() => route.meta.containedMainScroll === true);
const useViewportScroll = computed(() =>
  authed.value && !isAdminShell.value && !usesGuestSimpleLayout.value && !wideMain.value && !containedMainScroll.value
);
const appRootClass = computed(() => {
  if (isAdminShell.value) return "min-h-screen";
  const base = useViewportScroll.value || usesGuestSimpleLayout.value || !authed.value
    ? "min-h-[100dvh]"
    : "h-[100dvh] max-h-[100dvh]";
  /** Reserve the top safe area at the root level when the header is hidden. */
  const topSafe =
    usesGuestSimpleLayout.value
      ? "pt-[env(safe-area-inset-top,0px)]"
      : hideMobileChrome.value && authed.value
        ? "max-md:pt-[env(safe-area-inset-top,0px)]"
        : "";
  return [base, topSafe, usesGuestSimpleLayout.value ? "ui-simple-layout" : ""].filter(Boolean).join(" ");
});
const shellClass = computed(() => ({
  'ui-shell': true,
  'ui-shell--simple': usesGuestSimpleLayout.value,
  'ui-shell--guest': !authed.value && !usesGuestSimpleLayout.value,
  'ui-shell--wide': wideMain.value,
  'ui-shell--admin': isAdminShell.value,
  'ui-shell--contained': !useViewportScroll.value && authed.value && !usesGuestSimpleLayout.value,
}));
const mainFooterNavItems = computed(() => [
  { to: "/feed", label: t("app.nav.home"), icon: "home" as const },
  { to: "/search", label: t("app.nav.topics"), icon: "search" as const },
  { to: "/notifications", label: t("app.nav.notifications"), icon: "bell" as const },
  { to: "/messages", label: t("app.nav.messages"), icon: "message" as const },
]);
const mainClass = computed(() => ({
  'ui-main': true,
  'ui-main--simple': usesGuestSimpleLayout.value,
  'ui-main--wide': wideMain.value,
  'ui-main--admin': isAdminShell.value,
  'ui-main--contained': !useViewportScroll.value && authed.value && !usesGuestSimpleLayout.value,
  'ui-main--mobile-nav': authed.value && !isAdminShell.value && !usesGuestSimpleLayout.value && !hideMobileChrome.value,
}));
const isFeedRoute = computed(() => route.path === "/feed" || route.path === "/feed/scheduled");
const isSearchRoute = computed(() => route.path === "/search");
const isNotificationsRoute = computed(() => route.path === "/notifications");
const isMessagesRoute = computed(() => route.path === "/messages" || route.path.startsWith("/messages/"));
const sidebarComposeMode = computed<"normal" | "community">(() =>
  route.path.startsWith("/communities/") && route.path !== "/communities/new" ? "community" : "normal",
);
const sidebarComposeCommunityID = computed(() =>
  sidebarComposeMode.value === "community" ? String(route.params.id || "") : null,
);

function isFooterNavActive(path: string): boolean {
  if (path === "/feed") return isFeedRoute.value;
  if (path === "/search") return isSearchRoute.value;
  if (path === "/notifications") return isNotificationsRoute.value;
  if (path === "/messages") return isMessagesRoute.value;
  return false;
}

function footerNavBadge(path: string): number {
  if (path === "/notifications") return unreadNotificationCount.value;
  if (path === "/messages") return unreadDMCount.value;
  return 0;
}

async function loadMe() {
  const token = getAccessToken();
  if (!token) {
    me.value = null;
    return;
  }
  try {
    const u = await api<{
      id: string;
      email: string;
      handle: string;
      display_name?: string;
      badges?: string[];
      avatar_url?: string | null;
      is_site_admin?: boolean;
      fanclub_patreon_enabled?: boolean;
    }>("/api/v1/me", {
      method: "GET",
      token,
    });
    avatarImgFailed.value = false;
    me.value = {
      id: u.id,
      email: u.email,
      handle: u.handle ?? "",
      display_name: (u.display_name ?? "").trim() || displayNameFromEmail(u.email),
      badges: Array.isArray(u.badges) ? u.badges.map((badge) => String(badge)) : [],
      avatar_url: safeHttpURL(u.avatar_url) || null,
      is_site_admin: !!u.is_site_admin,
      fanclub_patreon_enabled: !!u.fanclub_patreon_enabled,
    };
    await refreshUnreadNotificationCount();
    await refreshUnreadDMCount();
  } catch {
    me.value = null;
  }
}

function stopNotifyStream() {
  disconnectNotifyStream?.();
  disconnectNotifyStream = null;
}

function stopDMStream() {
  disconnectDMStream?.();
  disconnectDMStream = null;
}

function showNotifyToast(msg: string) {
  if (notifyToastTimer) clearTimeout(notifyToastTimer);
  notifyToastMessageText.value = msg;
  notifyToastTimer = setTimeout(() => {
    notifyToastMessageText.value = "";
    notifyToastTimer = null;
  }, 5200);
}

function startNotifyStream() {
  stopNotifyStream();
  const token = getAccessToken();
  if (!token || !me.value) return;
  disconnectNotifyStream = connectNotifyStream({
    token,
    onPayload: (p: NotifyPayload) => {
      incrementUnreadNotificationCount();
      pingNotificationReceived();
      showNotifyToast(notifyToastMessage(p));
    },
  });
}

function startDMStream() {
  stopDMStream();
  const token = getAccessToken();
  if (!token || !me.value) return;
  disconnectDMStream = connectDMStream({
    token,
    onPayload: (p: DmStreamPayload) => {
      pingDMReceived(p);
      if (p.kind === "message" || p.kind === "federation_dm_invite" || p.kind === "federation_dm_message") {
        incrementUnreadDMCount();
      }
    },
  });
}

watch(
  () => route.fullPath,
  async () => {
    syncSearchQueryFromRoute();
    authed.value = !!getAccessToken();
    closeProfileMenu();
    mobileNavOpen.value = false;
    if (!authed.value) {
      me.value = null;
      stopNotifyStream();
      stopDMStream();
      return;
    }
    await loadMe();
  },
  { immediate: true },
);

watch(meHubTick, () => {
  if (authed.value) void loadMe();
});

watch(
  () => [me.value, authed.value] as const,
  () => {
    stopNotifyStream();
    stopDMStream();
    if (authed.value && me.value && getAccessToken()) {
      startNotifyStream();
      startDMStream();
    }
  },
  { immediate: true },
);

watch(
  () => operatorAnnouncements.value.length,
  () => {
    clampOperatorAnnouncementIndex();
    startOperatorAnnouncementSlider();
  },
  { immediate: true },
);

watch(
  themePreference,
  (next) => {
    persistThemePreference(next);
    syncTheme();
  },
  { immediate: true },
);

watch(
  themeModePreference,
  (next) => {
    persistThemeModePreference(next);
    syncTheme();
  },
  { immediate: true },
);

function closeProfileMenu() {
  profileMenuOpen.value = false;
}

function closeMobileNav() {
  const wasOpen = mobileNavOpen.value;
  mobileNavOpen.value = false;
  if (wasOpen) void nextTick(() => document.getElementById("mobile-account-button")?.focus());
}

function toggleMobileNav() {
  if (mobileNavOpen.value) return closeMobileNav();
  mobileNavOpen.value = true;
  void nextTick(() => document.querySelector<HTMLButtonElement>(".ui-drawer-close")?.focus({ preventScroll: true }));
}

function onAsideNavClick(ev: MouseEvent) {
  const t = ev.target as HTMLElement | null;
  if (t?.closest("a")) {
    closeMobileNav();
  }
}

function openSidebarCompose() {
  sidebarComposeOpen.value = true;
  closeMobileNav();
}

function onSidebarComposeSubmitted() {
  closeMobileNav();
}

function toggleProfileMenu() {
  profileMenuOpen.value = !profileMenuOpen.value;
}

async function logout() {
  closeProfileMenu();
  stopNotifyStream();
  stopDMStream();
  clearRememberedUnlockedIdentity();
  notifyToastMessageText.value = "";
  try {
    await api("/api/v1/auth/logout", { method: "POST" });
  } catch {
    /* ignore logout network failures */
  }
  clearTokens();
  router.push("/login");
}

function syncAuthStateFromStorage() {
  const loggedIn = !!getAccessToken();
  authed.value = loggedIn;
  if (!loggedIn) {
    me.value = null;
    stopNotifyStream();
    stopDMStream();
    clearRememberedUnlockedIdentity();
    if (route.meta.requiresAuth) {
      void router.replace({ path: "/login", query: { next: route.fullPath } });
    }
    return;
  }
  void loadMe();
}

function onStorage(ev: StorageEvent) {
  if (ev.key !== null && ev.key !== ACCESS) return;
  syncAuthStateFromStorage();
}

function onDocumentPointerDown(ev: PointerEvent) {
  if (!profileMenuOpen.value) return;
  const root = profileMenuRoot.value;
  if (root && !root.contains(ev.target as Node)) {
    closeProfileMenu();
  }
}

function onDocumentKeydown(ev: KeyboardEvent) {
  if (ev.key === "Escape") {
    closeProfileMenu();
    closeMobileNav();
  }
  if (ev.key === "Tab" && mobileNavOpen.value && window.innerWidth < 768) {
    const elements = Array.from(document.querySelectorAll<HTMLElement>("#app-sidebar a[href], #app-sidebar button:not(:disabled)"))
      .filter(el => el.getClientRects().length > 0);
    const first = elements[0], last = elements[elements.length - 1];
    if (ev.shiftKey && document.activeElement === first) { ev.preventDefault(); last?.focus(); }
    else if (!ev.shiftKey && document.activeElement === last) { ev.preventDefault(); first?.focus(); }
  }
}

let appHeaderResizeObserver: ResizeObserver | null = null;

onMounted(() => {
  themeMediaQuery = systemThemeMediaQuery();
  themeMediaQuery?.addEventListener("change", onSystemThemeChange);
  document.addEventListener("pointerdown", onDocumentPointerDown);
  document.addEventListener("keydown", onDocumentKeydown);
  window.addEventListener("storage", onStorage);
  void nextTick(syncAppHeaderOffset);
  if (typeof ResizeObserver !== "undefined") {
    appHeaderResizeObserver = new ResizeObserver(() => {
      syncAppHeaderOffset();
    });
    if (appHeaderEl.value) appHeaderResizeObserver.observe(appHeaderEl.value);
  }
  window.addEventListener("resize", syncAppHeaderOffset);
  window.visualViewport?.addEventListener("resize", syncAppHeaderOffset);
  void loadOperatorAnnouncements();
});

onBeforeUnmount(() => {
  stopNotifyStream();
  stopDMStream();
  themeMediaQuery?.removeEventListener("change", onSystemThemeChange);
  document.removeEventListener("pointerdown", onDocumentPointerDown);
  document.removeEventListener("keydown", onDocumentKeydown);
  window.removeEventListener("storage", onStorage);
  window.removeEventListener("resize", syncAppHeaderOffset);
  window.visualViewport?.removeEventListener("resize", syncAppHeaderOffset);
  stopOperatorAnnouncementSlider();
  appHeaderResizeObserver?.disconnect();
  appHeaderResizeObserver = null;
});

watch(
  () => route.fullPath,
  () => {
    void nextTick(syncAppHeaderOffset);
  },
  { flush: "post" },
);

function avatarInitials(email: string): string {
  const local = email.split("@")[0] ?? "";
  const cleaned = local.replace(/[^a-zA-Z0-9]/g, "");
  if (cleaned.length >= 2) {
    return cleaned.slice(0, 2).toUpperCase();
  }
  if (local.length >= 2) {
    return local.slice(0, 2).toUpperCase();
  }
  return (local[0] ?? "?").toUpperCase();
}
</script>

<template>
  <div
    class="ui-app-shell flex flex-col text-neutral-900"
    :class="appRootClass"
    :style="{ '--app-header-offset': appHeaderOffset, '--app-viewport-height': viewportHeight }"
  >
    <a href="#main-content" class="ui-skip-link">{{ $t('ux.skipContent') }}</a>
    <div
      v-if="notifyToastMessageText"
      class="fixed right-4 z-[200] max-w-sm rounded-xl border border-lime-200 bg-white px-4 py-3 text-sm text-neutral-900 shadow-lg ring-1 ring-black/5 max-md:top-[calc(1rem+env(safe-area-inset-top,0px))] md:top-4"
      role="status"
    >
      {{ notifyToastMessageText }}
    </div>
    <header v-if="!authed && !usesGuestSimpleLayout && !isAdminShell" class="ui-guest-header">
      <RouterLink to="/" aria-label="Glipz"><img :src="logoImg" alt="Glipz" class="ui-brand" /></RouterLink>
      <form class="ui-search" @submit.prevent="onGlobalSearchEnter">
        <Icon name="search" class="h-5 w-5" />
        <label class="sr-only" for="global-search-guest">{{ $t('app.search.label') }}</label>
        <input id="global-search-guest" v-model="searchQuery" type="search" :placeholder="$t('app.search.guestPlaceholder')" />
      </form>
      <RouterLink to="/login" class="ui-button ui-button--primary">{{ $t('app.guest.login') }}</RouterLink>
    </header>

    <div
      class="relative mx-auto flex w-full min-h-0 flex-1"
      :class="shellClass"
    >
      <div
        v-if="authed && !usesGuestSimpleLayout && !isAdminShell && mobileNavOpen"
        class="fixed inset-0 z-30 bg-black/40 md:hidden"
        aria-hidden="true"
        @click="closeMobileNav"
      />
      <aside
        v-if="authed && !usesGuestSimpleLayout && !isAdminShell"
        id="app-sidebar"
        class="ui-sidebar"
        :class="{ 'ui-sidebar--open': mobileNavOpen }"
        :role="mobileNavOpen ? 'dialog' : undefined"
        :aria-modal="mobileNavOpen ? true : undefined"
        :aria-label="$t('app.menu.main')"
      >
        <RouterLink
          to="/feed"
          class="ui-sidebar-brand"
          aria-label="Glipz ホーム"
          @click="closeMobileNav"
        >
          <img :src="logoImg" alt="Glipz" class="ui-brand" />
        </RouterLink>
        <button v-if="mobileNavOpen" type="button" class="ui-drawer-close ui-button ui-icon-button ui-button--ghost md:hidden" :aria-label="$t('app.menu.close')" @click="closeMobileNav"><Icon name="close" class="h-5 w-5" /></button>
        <nav class="ui-primary-nav" @click="onAsideNavClick">
          <RouterLink
            to="/feed"
            class="ui-nav-link"
            active-class="ui-nav-link--active"
          >
            <Icon name="home" class="h-5 w-5 shrink-0" />
            <span>{{ $t("app.nav.home") }}</span>
          </RouterLink>
          <RouterLink
            to="/search"
            class="ui-nav-link"
            active-class="ui-nav-link--active"
          >
            <Icon name="search" class="h-5 w-5 shrink-0" />
            <span>{{ $t("app.nav.topics") }}</span>
          </RouterLink>
          <RouterLink
            to="/notifications"
            class="ui-nav-link"
            active-class="ui-nav-link--active"
          >
            <span class="flex min-w-0 flex-1 items-center gap-3">
              <Icon name="bell" class="h-5 w-5 shrink-0" />
              <span class="truncate">{{ $t("app.nav.notifications") }}</span>
            </span>
            <span
              v-if="unreadNotificationCount > 0"
              class="inline-flex h-5 min-w-[1.25rem] shrink-0 items-center justify-center whitespace-nowrap rounded-full bg-red-500 px-1.5 text-[11px] font-bold leading-none text-white ring-2 ring-white"
            >
              {{ unreadNotificationCount > 99 ? "99+" : unreadNotificationCount }}
            </span>
          </RouterLink>
          <RouterLink
            to="/messages"
            class="ui-nav-link"
            :class="route.path === '/messages' || route.path.startsWith('/messages/') ? 'ui-nav-link--active' : ''"
          >
            <span class="flex min-w-0 flex-1 items-center gap-3">
              <Icon name="message" class="h-5 w-5 shrink-0" />
              <span class="truncate">{{ $t("app.nav.messages") }}</span>
            </span>
            <span
              v-if="unreadDMCount > 0"
              class="inline-flex h-5 min-w-[1.25rem] shrink-0 items-center justify-center whitespace-nowrap rounded-full bg-red-500 px-1.5 text-[11px] font-bold leading-none text-white ring-2 ring-white"
            >
              {{ unreadDMCount > 99 ? "99+" : unreadDMCount }}
            </span>
          </RouterLink>
          <RouterLink
            to="/bookmarks"
            class="ui-nav-link"
            active-class="ui-nav-link--active"
          >
            <Icon name="bookmark" class="h-5 w-5 shrink-0" />
            <span>{{ $t("app.nav.bookmarks") }}</span>
          </RouterLink>
          <RouterLink
            to="/communities"
            class="ui-nav-link"
            active-class="ui-nav-link--active"
          >
            <Icon name="hub" class="h-5 w-5 shrink-0" />
            <span>{{ $t("app.nav.communities") }}</span>
          </RouterLink>
          <RouterLink
            v-if="me?.handle"
            :to="`/@${me.handle}`"
            class="ui-nav-link"
            active-class="ui-nav-link--active"
          >
            <Icon name="user" class="h-5 w-5 shrink-0" />
            <span>{{ $t("app.nav.profile") }}</span>
          </RouterLink>
          <RouterLink
            to="/settings"
            class="ui-nav-link"
            active-class="ui-nav-link--active"
          >
            <Icon name="settings" class="h-5 w-5 shrink-0" />
            <span>{{ $t("app.nav.settings") }}</span>
          </RouterLink>
        </nav>

        <div class="mt-4 shrink-0 px-0.5">
          <button
            type="button"
            class="ui-button ui-button--primary ui-sidebar-compose w-full"
            @click="openSidebarCompose"
          >
            <Icon name="pencil" class="h-5 w-5" /><span>{{ $t("app.nav.post") }}</span>
          </button>
          <p class="mt-3 text-center text-[11px] font-medium uppercase tracking-wide text-neutral-400">
            APP VERSION {{ APP_VERSION }}
          </p>
        </div>

        <div ref="profileMenuRoot" class="ui-sidebar-account relative shrink-0">
          <button
            id="profile-menu-button"
            type="button"
            class="ui-account-button"
            :title="me?.email ?? $t('app.nav.account')"
            :aria-label="$t('app.menu.account')"
            :aria-expanded="profileMenuOpen"
            aria-haspopup="true"
            aria-controls="profile-menu"
            @click.stop="toggleProfileMenu"
          >
            <span
              class="relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full text-sm font-semibold ring-2 ring-lime-200"
              :class="
                me?.avatar_url && !avatarImgFailed
                  ? 'bg-neutral-200 text-white'
                  : 'bg-lime-500 text-white ring-lime-300'
              "
            >
              <img
                v-if="me?.avatar_url && !avatarImgFailed"
                :src="me.avatar_url"
                alt=""
                referrerpolicy="no-referrer"
                class="h-full w-full object-cover"
                @error="avatarImgFailed = true"
              />
              <span v-else-if="me?.email">{{ avatarInitials(me.email) }}</span>
              <span v-else class="text-xs">···</span>
            </span>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-1.5">
                <p class="truncate text-sm font-semibold text-neutral-900">
                  {{ me?.email ? me.display_name || displayNameFromEmail(me.email) : $t("app.loading") }}
                </p>
                <UserBadges v-if="me?.email" :badges="me?.badges" size="xs" />
              </div>
              <p class="truncate text-xs text-neutral-500">
                {{
                  me?.handle
                    ? currentDomain()
                      ? `@${me.handle}@${currentDomain()}`
                      : `@${me.handle}`
                    : ""
                }}
              </p>
            </div>
          </button>
          <div
            v-show="profileMenuOpen"
            id="profile-menu"
            role="menu"
            aria-labelledby="profile-menu-button"
            class="absolute bottom-full left-0 right-0 z-50 mb-1.5 min-w-[11rem] overflow-hidden rounded-lg border border-neutral-200 bg-white py-1 shadow-lg ring-1 ring-black/5"
          >
            <RouterLink
              to="/settings"
              role="menuitem"
              class="block px-4 py-2.5 text-sm text-neutral-800 hover:bg-lime-50"
              @click="closeProfileMenu"
            >
              {{ $t("app.nav.settings") }}
            </RouterLink>
            <RouterLink
              v-if="me?.is_site_admin"
              to="/admin"
              role="menuitem"
              class="block border-t border-neutral-200 px-4 py-2.5 text-sm text-neutral-800 hover:bg-lime-50"
              @click="closeProfileMenu"
            >
              {{ $t("app.nav.adminControlPanel") }}
            </RouterLink>
            <button
              type="button"
              role="menuitem"
              class="w-full border-t border-neutral-200 px-4 py-2.5 text-left text-sm text-neutral-800 hover:bg-lime-50"
              @click="logout"
            >
              {{ $t("app.nav.logout") }}
            </button>
          </div>
        </div>
      </aside>
      <main id="main-content" tabindex="-1"
        class="min-w-0"
        :class="mainClass"
        :inert="mobileNavOpen || undefined"
      >
        <header v-if="authed && !usesGuestSimpleLayout && !isAdminShell && !hideAppHeader" ref="appHeaderEl" class="ui-content-header">
          <div v-if="!hideMobileChrome" class="ui-mobile-topbar md:hidden">
            <RouterLink to="/feed" aria-label="Glipz"><img :src="logoImg" alt="Glipz" class="ui-brand" /></RouterLink>
            <button id="mobile-account-button" class="ui-mobile-account" type="button" :aria-label="$t('app.menu.account')" :aria-expanded="mobileNavOpen" aria-controls="app-sidebar" @click="toggleMobileNav">
              <img v-if="me?.avatar_url && !avatarImgFailed" :src="me.avatar_url" alt="" @error="avatarImgFailed = true" />
              <span v-else>{{ me?.email ? avatarInitials(me.email) : '?' }}</span>
            </button>
          </div>
          <h1 v-if="isFeedRoute" class="ui-feed-heading hidden md:block">{{ $t('app.nav.home') }}</h1>
          <form v-if="isSearchRoute" class="ui-search ui-content-search xl:hidden" @submit.prevent="onGlobalSearchEnter">
            <Icon name="search" class="h-5 w-5" />
            <label class="sr-only" for="global-search-tablet">{{ $t('app.search.label') }}</label>
            <input id="global-search-tablet" v-model="searchQuery" type="search" :placeholder="$t('app.search.placeholder')" />
          </form>
          <div id="app-view-header-slot-desktop" class="ui-view-header hidden md:block" />
          <div id="app-view-header-slot-mobile" class="ui-view-header md:hidden" />
        </header>
        <RouterView />
      </main>
      <SidebarComposeModal
        v-if="authed"
        v-model:open="sidebarComposeOpen"
        :mode="sidebarComposeMode"
        :community-id="sidebarComposeCommunityID"
        :viewer-email="me?.email"
        :viewer-handle="me?.handle"
        :viewer-avatar-url="me?.avatar_url"
        :patreon-enabled="!!me?.fanclub_patreon_enabled"
        @submitted="onSidebarComposeSubmitted"
      />
      <aside
        v-if="authed && !usesGuestSimpleLayout && !isAdminShell && !hideRightAside"
        class="ui-right-sidebar"
        :aria-label="$t('app.menu.announcementsAndPolicies')"
      >
        <form class="ui-search" @submit.prevent="onGlobalSearchEnter">
          <Icon name="search" class="h-5 w-5" />
          <label class="sr-only" for="global-search">{{ $t('app.search.label') }}</label>
          <input id="global-search" v-model="searchQuery" type="search" :placeholder="$t('app.search.placeholder')" />
        </form>
        <section v-if="currentOperatorAnnouncement">
          <div class="flex items-center justify-between gap-3">
            <h2 class="text-xs font-semibold uppercase tracking-wide text-neutral-500">{{ $t("app.announcements.heading") }}</h2>
            <p v-if="hasMultipleOperatorAnnouncements" class="text-[11px] tabular-nums text-neutral-400">
              {{ operatorAnnouncementIndex + 1 }} / {{ operatorAnnouncements.length }}
            </p>
          </div>
          <div v-if="currentOperatorAnnouncement" class="mt-2 overflow-hidden rounded-xl border border-lime-100 bg-lime-50/60 text-sm shadow-sm">
            <div class="p-3">
              <p class="font-semibold text-neutral-900">{{ currentOperatorAnnouncement.title }}</p>
              <p class="mt-1.5 leading-relaxed text-neutral-700">{{ currentOperatorAnnouncement.body }}</p>
              <p class="mt-2 text-[11px] text-lime-700/70">{{ currentOperatorAnnouncement.date }}</p>
            </div>
            <div v-if="hasMultipleOperatorAnnouncements" class="flex items-center justify-between border-t border-lime-100 bg-white/65 px-2 py-1.5">
              <button
                type="button"
                class="rounded-full px-2 py-1 text-sm font-semibold text-lime-800 hover:bg-lime-100"
                :aria-label="$t('app.announcements.previous')"
                @click="showPreviousOperatorAnnouncement"
              >
                ‹
              </button>
              <div class="flex items-center gap-1.5">
                <button
                  v-for="(_, index) in operatorAnnouncements"
                  :key="index"
                  type="button"
                  class="h-1.5 rounded-full transition-all"
                  :class="index === operatorAnnouncementIndex ? 'w-5 bg-lime-600' : 'w-1.5 bg-neutral-300 hover:bg-lime-300'"
                  :aria-label="$t('app.announcements.goTo', { n: index + 1 })"
                  :aria-current="index === operatorAnnouncementIndex ? 'true' : undefined"
                  @click="showOperatorAnnouncement(index)"
                />
              </div>
              <button
                type="button"
                class="rounded-full px-2 py-1 text-sm font-semibold text-lime-800 hover:bg-lime-100"
                :aria-label="$t('app.announcements.next')"
                @click="showNextOperatorAnnouncement"
              >
                ›
              </button>
            </div>
          </div>
          <p v-else class="mt-2 text-sm text-neutral-500">{{ $t("app.announcements.empty") }}</p>
        </section>
        <SidebarWidgetHost placement="right-sidebar" :viewer="sidebarWidgetViewer" />
        <nav class="border-t border-neutral-200 pt-3 text-[12px] leading-relaxed" :aria-label="$t('app.menu.policyLinks')">
          <div class="flex flex-wrap items-center gap-x-1 gap-y-0.5 text-neutral-400">
            <a
              v-if="legalLink('terms').external"
              :href="legalLink('terms').href"
              target="_blank"
              rel="noopener noreferrer"
              class="rounded px-0.5 text-neutral-600 hover:text-lime-800 hover:underline"
            >
              {{ $t("app.links.terms") }}
            </a>
            <RouterLink
              v-else
              :to="legalLink('terms').href"
              class="rounded px-0.5 text-neutral-600 hover:text-lime-800 hover:underline"
            >
              {{ $t("app.links.terms") }}
            </RouterLink>
            <span aria-hidden="true">｜</span>
            <a
              v-if="legalLink('privacy').external"
              :href="legalLink('privacy').href"
              target="_blank"
              rel="noopener noreferrer"
              class="rounded px-0.5 text-neutral-600 hover:text-lime-800 hover:underline"
            >
              {{ $t("app.links.privacy") }}
            </a>
            <RouterLink
              v-else
              :to="legalLink('privacy').href"
              class="rounded px-0.5 text-neutral-600 hover:text-lime-800 hover:underline"
            >
              {{ $t("app.links.privacy") }}
            </RouterLink>
            <span aria-hidden="true">｜</span>
            <a
              v-if="legalLink('nsfw').external"
              :href="legalLink('nsfw').href"
              target="_blank"
              rel="noopener noreferrer"
              class="rounded px-0.5 text-neutral-600 hover:text-lime-800 hover:underline"
            >
              {{ $t("app.links.nsfw") }}
            </a>
            <RouterLink
              v-else
              :to="legalLink('nsfw').href"
              class="rounded px-0.5 text-neutral-600 hover:text-lime-800 hover:underline"
            >
              {{ $t("app.links.nsfw") }}
            </RouterLink>
            <span aria-hidden="true">｜</span>
            <RouterLink
              to="/federation/guidelines"
              class="rounded px-0.5 text-neutral-600 hover:text-lime-800 hover:underline"
            >
              {{ $t("app.links.federation") }}
            </RouterLink>
            <span aria-hidden="true">｜</span>
            <RouterLink
              to="/legal/api-guidelines"
              class="rounded px-0.5 text-neutral-600 hover:text-lime-800 hover:underline"
            >
              {{ $t("app.links.apiReference") }}
            </RouterLink>
          </div>
        </nav>
      </aside>
    </div>
    <nav
      v-if="authed && !usesGuestSimpleLayout && !isAdminShell && !hideMobileChrome"
      class="ui-bottom-nav md:hidden"
      :aria-label="$t('app.menu.mobileFooter')"
    >
      <div class="grid grid-cols-4">
        <RouterLink
          v-for="item in mainFooterNavItems"
          :key="item.to"
          :to="item.to"
          class="ui-bottom-link"
          :class="isFooterNavActive(item.to) ? 'text-lime-700' : 'text-neutral-500 hover:text-neutral-800'"
          :aria-label="item.label"
          :aria-current="isFooterNavActive(item.to) ? 'page' : undefined"
        >
          <Icon :name="item.icon" class="h-6 w-6" />
          <span class="ui-bottom-label">{{ item.label }}</span>
          <span
            v-if="footerNavBadge(item.to) > 0"
            class="absolute left-1/2 top-2 ml-2 inline-flex h-5 min-w-[1.25rem] items-center justify-center whitespace-nowrap rounded-full bg-red-500 px-1.5 text-[11px] font-bold leading-none text-white ring-2 ring-white"
          >
            {{ footerNavBadge(item.to) > 99 ? "99+" : footerNavBadge(item.to) }}
          </span>
        </RouterLink>
      </div>
    </nav>
  </div>
</template>
