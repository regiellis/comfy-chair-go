import { app } from '/scripts/app.js';
app.registerExtension({
  name: '{{NodeNameLower}}.settings',
  settings: [
    { id: '{{NodeNameLower}}.auto_show', type: 'boolean', name: 'Auto show {{NodeName}} panel', defaultValue: true },
  ],
});
