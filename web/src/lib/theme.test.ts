// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { THEME_PRESETS, applyTheme } from './theme';
import { contrastRatio, readableForeground } from './contrast';
describe('theme readability', () => {
  for (const preset of THEME_PRESETS) for (const mode of ['light', 'dark'] as const) {
    it(`${preset.value} ${mode} keeps text and primary actions readable`, () => {
      const p = preset[mode];
      for (const surface of [p.background, p.surface, p.surfaceMuted]) {
        expect(contrastRatio(p.text, surface)).toBeGreaterThanOrEqual(4.5);
        expect(contrastRatio(p.textMuted, surface)).toBeGreaterThanOrEqual(4.5);
      }
      expect(contrastRatio(p.accentStrong, readableForeground(p.accentStrong, p.accentContrast))).toBeGreaterThanOrEqual(4.5);
      applyTheme(preset.value, mode);
      expect(document.documentElement.classList.contains('dark')).toBe(mode === 'dark');
    });
  }
});
