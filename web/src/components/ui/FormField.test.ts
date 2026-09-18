// @vitest-environment jsdom
import { afterEach, expect, it } from 'vitest';
import { createApp, h, nextTick, type App } from 'vue';
import FormField from './FormField.vue';
import Button from './Button.vue';
let app: App | undefined;
afterEach(() => { app?.unmount(); document.body.innerHTML = ''; });
it('connects input names, instructions and validation errors for assistive technology', async () => {
  const root = document.createElement('div'); document.body.append(root);
  app = createApp({ render: () => h(FormField, { id: 'email', label: 'Email', hint: 'Use your email', error: 'Invalid email' }, {
    default: (field: {id: string; describedby: string; invalid: boolean}) => h('input', { id: field.id, 'aria-describedby': field.describedby, 'aria-invalid': field.invalid }),
  }) });
  app.mount(root); await nextTick();
  const input = root.querySelector('input')!;
  expect(input.labels?.[0].textContent).toBe('Email');
  expect(input.getAttribute('aria-describedby')).toBe('email-hint email-error');
  expect(input.getAttribute('aria-invalid')).toBe('true');
  expect(root.querySelector('[role=alert]')?.textContent).toBe('Invalid email');
});
it('prevents duplicate actions while a request is in progress', () => {
  const root = document.createElement('div'); document.body.append(root);
  let calls = 0;
  app = createApp({ render: () => h(Button, { loading: true, onClick: () => calls++ }, () => 'Submit') });
  app.mount(root); const button = root.querySelector('button')!; button.click();
  expect(calls).toBe(0); expect(button.disabled).toBe(true); expect(button.getAttribute('aria-busy')).toBe('true');
});
