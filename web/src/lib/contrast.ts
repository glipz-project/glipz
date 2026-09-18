function luminance(hex: string): number {
  const color = hex.replace('#', '');
  const channels = [0, 2, 4].map(offset => {
    const value = parseInt(color.slice(offset, offset + 2), 16) / 255;
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return channels[0] * .2126 + channels[1] * .7152 + channels[2] * .0722;
}
export function contrastRatio(a: string, b: string): number {
  const x = luminance(a), y = luminance(b);
  return (Math.max(x, y) + .05) / (Math.min(x, y) + .05);
}
export function readableForeground(background: string, preferred: string): string {
  if (contrastRatio(background, preferred) >= 4.5) return preferred;
  return contrastRatio(background, '#ffffff') > contrastRatio(background, '#000000') ? '#ffffff' : '#000000';
}
