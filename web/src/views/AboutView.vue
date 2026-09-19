<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, type Ref } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink, useRouter } from "vue-router";
import logoDarkImg from "../assets/glipz-dark.png";
import logoLightImg from "../assets/glipz-light.png";
import EmptyState from "../components/ui/EmptyState.vue";
import PostTimeline from "../components/PostTimeline.vue";
import { getOperatorAnnouncements } from "../data/operatorAnnouncements";
import { setLocale, supportedLocaleOptions, type AppLocale } from "../i18n";
import { APP_VERSION, FEDERATION_PROTOCOL_VERSION } from "../lib/appInfo";
import { connectFeedStream, fetchFeedItem, fetchPublicFeedItems, type FeedPubPayload } from "../lib/feedStream";
import { fetchPublicInstanceSettings, type OperatorAnnouncement } from "../lib/instanceSettings";
import { legalDocumentLink, type LegalDocumentURLSettings } from "../lib/legalDocumentLinks";
import { readStoredThemeModePreference, resolveTheme, type ResolvedTheme } from "../lib/theme";
import type { TimelinePost } from "../types/timeline";

const router = useRouter();
const { t, tm, locale } = useI18n();
const resolvedTheme = inject<Ref<ResolvedTheme>>("resolvedTheme");
const fallbackTheme = resolveTheme(readStoredThemeModePreference());
const logoImg = computed(() => (resolvedTheme?.value ?? fallbackTheme) === "dark" ? logoDarkImg : logoLightImg);
const publicTimelineItems = ref<TimelinePost[]>([]);
const publicTimelineLoading = ref(true);
const publicTimelineError = ref("");
let disconnectPublicFeed: (() => void) | null = null;
const primaryFeatures = computed(() => [
  { title: t("landing.federationTitle"), body: t("landing.federationBody") },
  ...(tm("about.features") as Array<{ title: string; body: string }>).slice(1),
]);
const localeOptions = computed(() =>
  supportedLocaleOptions.map((option) => ({ value: option.value, label: t(option.labelKey) })),
);
const trustPoints = computed(() => {
  const points = tm("about.publicInfo.points") as string[];
  return points.map((point) =>
    point
      .replace(APP_VERSION, APP_VERSION)
      .replace(FEDERATION_PROTOCOL_VERSION, FEDERATION_PROTOCOL_VERSION),
  );
});
const legalDocumentUrls = ref<LegalDocumentURLSettings>({});
const quickLinks = computed(() =>
  (tm("about.quickLinks") as Array<{ to: string; title: string; body: string }>).map((item) => {
    if (item.to === "/legal/terms") {
      const link = legalDocumentLink(legalDocumentUrls.value, "terms");
      return { ...item, to: link.href, external: link.external };
    }
    if (item.to === "/legal/privacy") {
      const link = legalDocumentLink(legalDocumentUrls.value, "privacy");
      return { ...item, to: link.href, external: link.external };
    }
    if (item.to === "/legal/nsfw-guidelines") {
      const link = legalDocumentLink(legalDocumentUrls.value, "nsfw");
      return { ...item, to: link.href, external: link.external };
    }
    return { ...item, external: false };
  }),
);
const operatorAnnouncements = ref<OperatorAnnouncement[]>(getOperatorAnnouncements());

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

async function loadPublicTimeline() {
  publicTimelineLoading.value = true;
  publicTimelineError.value = "";
  try {
    publicTimelineItems.value = await fetchPublicFeedItems();
  } catch (e: unknown) {
    publicTimelineError.value = e instanceof Error ? e.message : t("notifications.generic");
  } finally {
    publicTimelineLoading.value = false;
  }
}

function stopPublicTimelineStream() {
  disconnectPublicFeed?.();
  disconnectPublicFeed = null;
}

async function handlePublicFeedPayload(p: FeedPubPayload) {
  if (p.kind === "post_deleted") {
    publicTimelineItems.value = publicTimelineItems.value.filter((it) => it.id !== p.post_id);
    return;
  }
  if (p.kind !== "post_created" && p.kind !== "post_updated") return;
  const row = await fetchFeedItem(p.post_id, null);
  if (!row || row.visibility !== "public") {
    publicTimelineItems.value = publicTimelineItems.value.filter((it) => it.id !== p.post_id);
    return;
  }
  const idx = publicTimelineItems.value.findIndex((it) => it.id === row.id);
  if (idx >= 0) {
    const next = publicTimelineItems.value.slice();
    next[idx] = row;
    publicTimelineItems.value = next;
    return;
  }
  publicTimelineItems.value = [row, ...publicTimelineItems.value].slice(0, 30);
}

function startPublicTimelineStream() {
  stopPublicTimelineStream();
  disconnectPublicFeed = connectFeedStream({
    scope: "all",
    public: true,
    onPayload: (p) => void handlePublicFeedPayload(p),
  });
}

function goLogin() {
  void router.push("/login");
}

function selectLocale(next: AppLocale) {
  setLocale(next);
}

onMounted(async () => {
  await loadOperatorAnnouncements();
  await loadPublicTimeline();
  startPublicTimelineStream();
});

onBeforeUnmount(() => {
  stopPublicTimelineStream();
});
</script>

<template>
  <div class="about-page">
    <header class="landing-header landing-container">
      <RouterLink to="/" aria-label="Glipz" class="landing-brand"><img :src="logoImg" alt="Glipz" /></RouterLink>
      <nav class="landing-nav" :aria-label="$t('landing.nav')">
        <a href="#about-features">{{ $t('landing.nav') }}</a>
        <a href="#public-timeline">{{ $t('about.publicTimeline.title') }}</a>
      </nav>
      <div class="landing-header-actions">
        <label class="sr-only" for="about-locale-select">{{ $t('app.locale.heading') }}</label>
        <select id="about-locale-select" :value="locale" @change="selectLocale(($event.target as HTMLSelectElement).value as AppLocale)">
          <option v-for="opt in localeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
        </select>
        <RouterLink to="/login" class="landing-login">{{ $t('common.actions.login') }}</RouterLink>
      </div>
    </header>

    <section class="landing-hero landing-container" aria-labelledby="landing-title">
      <div class="landing-hero-copy">
        <h1 id="landing-title"><span v-for="line in ($tm('landing.title') as string[])" :key="line">{{ line }}</span></h1>
        <p class="landing-lead">{{ $t('landing.lead') }}</p>
        <RouterLink to="/register" class="ui-button ui-button--primary landing-primary">{{ $t('common.actions.createAccount') }}</RouterLink>
        <a href="#public-timeline" class="landing-text-link">{{ $t('landing.explore') }} <span aria-hidden="true">↓</span></a>
      </div>
      <figure class="landing-network" role="img" :aria-label="$t('landing.diagram')">
        <svg viewBox="0 0 520 460" fill="none" aria-hidden="true" focusable="false">
          <path class="network-link" d="M170 110 Q260 20 350 110 M420 200 Q425 320 315 360 M100 200 Q95 320 205 360" />
          <circle class="network-node" cx="104" cy="151" r="78" />
          <circle class="network-node" cx="416" cy="151" r="78" />
          <circle class="network-home" cx="260" cy="352" r="80" />
          <circle class="network-dot" cx="260" cy="66" r="9" />
          <circle class="network-dot" cx="126" cy="291" r="9" />
          <circle class="network-dot" cx="394" cy="291" r="9" />
          <text x="104" y="162" text-anchor="middle">Glipz</text>
          <text x="416" y="162" text-anchor="middle">Glipz</text>
          <text class="network-home-text" x="260" y="365" text-anchor="middle">Glipz</text>
        </svg>
      </figure>
    </section>

    <section id="about-features" class="landing-features landing-container" :aria-label="$t('landing.nav')">
      <article v-for="(feature, index) in primaryFeatures" :key="feature.title">
        <span class="landing-feature-number" aria-hidden="true">0{{ index + 1 }}</span>
        <h2>{{ feature.title }}</h2><p>{{ feature.body }}</p>
      </article>
    </section>

    <section id="public-timeline" class="landing-band" aria-labelledby="landing-conversation">
      <div class="landing-container landing-conversations">
        <div>
          <h2 id="landing-conversation">{{ $t('landing.conversation') }}</h2>
          <p class="landing-section-description">{{ $t('about.publicTimeline.description') }}</p>
          <RouterLink to="/register" class="landing-text-link">{{ $t('common.actions.joinAndPost') }} <span aria-hidden="true">→</span></RouterLink>
        </div>
        <div class="landing-timeline">
          <h3>{{ $t('about.publicTimeline.title') }}</h3>
          <EmptyState v-if="publicTimelineError" :title="publicTimelineError" :action-label="$t('ux.retry')" @action="loadPublicTimeline" />
          <EmptyState v-else-if="publicTimelineLoading && !publicTimelineItems.length" :title="$t('about.publicTimeline.loading')" busy />
          <EmptyState v-else-if="!publicTimelineItems.length" :title="$t('about.publicTimeline.empty')" :description="$t('ux.emptyPublic')" />
          <div v-else class="landing-timeline-scroll" tabindex="0" :aria-label="$t('about.publicTimeline.title')">
            <PostTimeline :items="publicTimelineItems" :action-busy="null" :embed-thread-replies="false"
              @reply="goLogin" @toggle-reaction="goLogin" @toggle-bookmark="goLogin" @toggle-repost="goLogin" @share="goLogin" />
          </div>
        </div>
      </div>
    </section>

    <section class="landing-info landing-container" aria-labelledby="landing-before">
      <h2 id="landing-before">{{ $t('landing.before') }}</h2>
      <p class="landing-section-description">{{ $t('about.publicInfo.description') }}</p>
      <div class="landing-info-columns">
        <div>
          <ul class="landing-trust"><li v-for="point in trustPoints" :key="point">{{ point }}</li></ul>
          <div class="landing-links">
            <template v-for="item in quickLinks" :key="item.to">
              <a v-if="item.external" :href="item.to" target="_blank" rel="noopener noreferrer">
                <span>{{ item.title }}<small>{{ item.body }}</small></span><span aria-hidden="true">↗</span>
              </a>
              <RouterLink v-else :to="item.to">
                <span>{{ item.title }}<small>{{ item.body }}</small></span><span aria-hidden="true">→</span>
              </RouterLink>
            </template>
          </div>
        </div>
        <div class="landing-notices">
          <h3>{{ $t('about.publicInfo.noticesTitle') }}</h3>
          <template v-if="operatorAnnouncements.length">
            <article v-for="item in operatorAnnouncements.slice(0, 3)" :key="item.id">
              <time>{{ item.date }}</time><h4>{{ item.title }}</h4><p>{{ item.body }}</p>
            </article>
          </template>
          <p v-else>{{ $t('about.publicInfo.empty') }}</p>
          <div class="landing-federation-note">
            <h3>{{ $t('about.activityPubDifference.title') }}</h3>
            <p>{{ $t('about.activityPubDifference.body') }}</p>
          </div>
        </div>
      </div>
    </section>

    <section class="landing-cta landing-band">
      <div class="landing-container">
        <h2>{{ $t('landing.cta') }}</h2>
        <p>{{ $t('about.cta.description') }}</p>
        <RouterLink to="/register" class="ui-button ui-button--primary landing-primary">{{ $t('common.actions.startNow') }}</RouterLink>
      </div>
    </section>
    <footer class="landing-footer landing-container">
      <img :src="logoImg" alt="Glipz" />
      <p>{{ $t('common.labels.app') }} {{ APP_VERSION }} <span aria-hidden="true">·</span> {{ $t('common.labels.federation') }} {{ FEDERATION_PROTOCOL_VERSION }}</p>
    </footer>
  </div>
</template>

<style scoped>
.about-page { color:rgb(var(--theme-text)); margin:-40px -24px; width:calc(100% + 48px); }
.landing-container { width:min(100% - 80px,1200px); margin-inline:auto; }
.landing-header { display:flex; align-items:center; gap:40px; min-height:104px; }
.landing-brand { flex:none; }
.landing-brand img { width:152px; height:auto; }
.landing-nav,.landing-header-actions { display:flex; align-items:center; gap:28px; font-size:14px; }
.landing-header-actions { margin-left:auto; gap:20px; }
.landing-header-actions select { max-width:124px; min-height:44px; background:transparent; color:inherit; border:0; border-bottom:1px solid rgb(var(--color-frame)); padding:8px; }
.landing-header-actions option { background:rgb(var(--theme-surface)); }
.landing-nav a,.landing-login { display:inline-flex; align-items:center; min-height:44px; }
.landing-login { border-left:1px solid rgb(var(--color-frame)); padding-left:24px; }
.landing-nav a:hover,.landing-login:hover { color:rgb(var(--theme-accent-text)); }
.landing-hero { display:grid; grid-template-columns:1.1fr 1fr; align-items:center; gap:32px; padding-block:64px 72px; }
.landing-hero h1 { font-size:clamp(36px,4.3vw,60px); line-height:1.27; letter-spacing:-.055em; font-weight:800; margin:0; }
.landing-hero h1 span { display:block; }
.landing-lead { font-size:17px; line-height:1.85; max-width:540px; margin:28px 0; }
.landing-primary { min-height:52px; padding:12px 32px; border-radius:8px; font-size:16px; }
.landing-text-link { display:table; margin-top:20px; padding-block:8px; min-height:44px; border-bottom:1px solid currentColor; font-size:15px; }
.landing-text-link span { margin-left:12px; }
.landing-text-link:hover { color:rgb(var(--theme-accent-text)); }
.landing-network { margin:0; }
.landing-network svg { display:block; width:100%; height:auto; }
.network-link { stroke:rgb(var(--theme-text-muted)); stroke-width:1.5; }
.network-node { fill:rgb(var(--theme-surface)); stroke:rgb(var(--theme-accent-bg-strong)); stroke-width:2; }
.network-home { fill:rgb(var(--theme-text)); }
.network-dot { fill:rgb(var(--theme-accent-bg-strong)); stroke:rgb(var(--theme-surface)); stroke-width:4; }
.landing-network text { fill:rgb(var(--theme-text)); font-size:30px; font-weight:750; letter-spacing:-1px; }
.landing-network .network-home-text { fill:rgb(var(--theme-surface)); font-size:34px; }
.landing-features { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); padding-block:40px 56px; border-top:1px solid rgb(var(--color-frame)); scroll-margin-top:24px; }
.landing-features article { padding:0 32px; border-left:1px solid rgb(var(--color-frame)); }
.landing-features article:first-child { padding-left:0; border:0; }
.landing-features article:last-child { padding-right:0; }
.landing-feature-number { color:rgb(var(--theme-accent-text)); font-size:24px; font-weight:700; }
.landing-features h2 { font-size:21px; font-weight:700; margin:8px 0 12px; }
.landing-features p { font-size:15px; line-height:1.85; color:rgb(var(--theme-text-muted)); }
.landing-band { background:rgb(var(--theme-surface-muted)); }
.landing-conversations { display:grid; grid-template-columns:1fr 1.1fr; gap:64px; padding-block:64px; align-items:center; }
.landing-conversations h2 { font-size:clamp(30px,3.4vw,46px); font-weight:750; line-height:1.4; letter-spacing:-.04em; max-width:420px; }
.landing-section-description { margin-top:20px; line-height:1.85; color:rgb(var(--theme-text-muted)); }
.landing-timeline { min-width:0; border:1px solid rgb(var(--color-frame)); border-radius:12px; background:rgb(var(--theme-surface)); overflow:hidden; }
.landing-timeline h3 { padding:24px; font-size:18px; font-weight:700; border-bottom:1px solid rgb(var(--color-frame)); }
.landing-timeline :deep(.ui-empty) { min-height:240px; display:flex; flex-direction:column; justify-content:center; }
.landing-timeline-scroll { max-height:440px; overflow:auto; }
.landing-info { padding-block:64px; }
.landing-info h2,.landing-cta h2 { font-size:clamp(26px,3vw,38px); font-weight:750; letter-spacing:-.03em; line-height:1.45; }
.landing-info-columns { display:grid; grid-template-columns:1fr 1fr; gap:64px; margin-top:36px; }
.landing-trust { font-size:14px; color:rgb(var(--theme-text-muted)); line-height:1.8; list-style:disc; padding-left:20px; margin-bottom:24px; }
.landing-trust li + li { margin-top:8px; }
.landing-links a { display:flex; align-items:center; justify-content:space-between; gap:20px; padding:16px 0; border-top:1px solid rgb(var(--color-frame)); font-size:15px; font-weight:600; }
.landing-links a:hover { color:rgb(var(--theme-accent-text)); }
.landing-links small { display:block; font-size:12px; font-weight:400; color:rgb(var(--theme-text-muted)); margin-top:4px; }
.landing-notices { padding-left:40px; border-left:1px solid rgb(var(--color-frame)); }
.landing-notices h3 { font-size:18px; font-weight:650; line-height:1.6; }
.landing-notices article { border-top:1px solid rgb(var(--color-frame)); padding:20px 0; margin-top:16px; }
.landing-notices time { font-size:12px; color:rgb(var(--theme-text-muted)); }
.landing-notices h4 { font-size:15px; font-weight:650; margin-top:8px; }
.landing-notices p { margin-top:12px; font-size:14px; line-height:1.9; color:rgb(var(--theme-text-muted)); }
.landing-federation-note { margin-top:32px; border-top:1px solid rgb(var(--color-frame)); padding-top:24px; }
.landing-cta { text-align:center; padding-block:56px; }
.landing-cta p { margin:16px auto 24px; max-width:680px; color:rgb(var(--theme-text-muted)); line-height:1.85; }
.landing-footer { display:flex; gap:24px; justify-content:space-between; align-items:center; padding-block:28px; }
.landing-footer img { width:112px; height:auto; }
.landing-footer p { font-size:12px; color:rgb(var(--theme-text-muted)); }
@media (max-width:1023px) {
  .landing-nav { display:none; }
  .landing-conversations,.landing-info-columns { gap:32px; }
  .landing-features article { padding-inline:20px; }
}
@media (max-width:767px) {
  .about-page { margin:-32px -20px; width:calc(100% + 40px); }
  .landing-container { width:calc(100% - 40px); }
  .landing-header { min-height:84px; gap:16px; flex-wrap:wrap; padding-block:12px; }
  .landing-brand img { width:112px; }
  .landing-header-actions { gap:12px; font-size:12px; }
  .landing-header-actions select { max-width:88px; padding-inline:0; }
  .landing-login { padding-left:12px; }
  .landing-hero { grid-template-columns:1fr; padding-block:36px 40px; gap:24px; }
  .landing-hero h1 { font-size:clamp(32px,8.5vw,48px); }
  .landing-lead { font-size:15px; margin:20px 0 24px; }
  .landing-network { width:min(100%,360px); margin-inline:auto; }
  .landing-features { grid-template-columns:1fr; padding-block:0 32px; }
  .landing-features article,.landing-features article:first-child,.landing-features article:last-child { padding:24px 0; border:0; border-bottom:1px solid rgb(var(--color-frame)); }
  .landing-features h2 { font-size:20px; }
  .landing-conversations,.landing-info-columns { grid-template-columns:1fr; gap:32px; }
  .landing-conversations,.landing-info { padding-block:40px; }
  .landing-notices { padding:0; border:0; }
  .landing-footer { flex-wrap:wrap; gap:16px; }
  .landing-cta { padding-block:40px; }
}
</style>
