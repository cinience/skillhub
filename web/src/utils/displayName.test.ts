import test from 'node:test';
import assert from 'node:assert/strict';
import { formatDisplayName } from './displayName.ts';

test('keeps a short display name unchanged', () => {
  const result = formatDisplayName('Twilio', 'twilio-api');

  assert.equal(result.text, 'Twilio');
  assert.equal(result.tooltip, undefined);
  assert.equal(result.truncated, false);
});

test('falls back to slug when display name is blank', () => {
  const result = formatDisplayName('   ', 'social-content');

  assert.equal(result.text, 'social-content');
  assert.equal(result.tooltip, undefined);
  assert.equal(result.truncated, false);
});

test('truncates an overly long display name and keeps full text in tooltip', () => {
  const longDisplayName = 'My goal is to support the community and continue creating more useful tools. If these automations prove to be very helpful to you.';

  const result = formatDisplayName(longDisplayName, 'social-content', 50);

  assert.equal(result.text, 'My goal is to support the community and continue…');
  assert.equal(result.tooltip, longDisplayName);
  assert.equal(result.truncated, true);
});
