/**
 * {{NodeName}} Settings Extension for ComfyUI
 *
 * This module registers settings for the {{NodeName}} node within ComfyUI's
 * settings panel. Users can configure these settings through the ComfyUI
 * interface (typically accessed via the gear icon or settings menu).
 *
 * Template Variables (replaced during node generation):
 * - {{NodeName}}: The display name of your node (e.g., "MyAwesomeNode")
 * - {{NodeNameLower}}: Lowercase version used for setting IDs (e.g., "myawesomenode")
 *
 * Adding New Settings:
 * To add more settings, add objects to the 'settings' array with these properties:
 * - id: Unique identifier (format: '{{NodeNameLower}}.setting_name')
 * - type: Setting type ('boolean', 'number', 'slider', 'combo', 'text')
 * - name: Display name shown in the settings panel
 * - defaultValue: Default value for the setting
 *
 * Additional properties for specific types:
 * - For 'slider': min, max, step
 * - For 'combo': options (array of choices)
 *
 * @example
 * // Example settings you might add:
 * {
 *   id: '{{NodeNameLower}}.api_timeout',
 *   type: 'number',
 *   name: 'API Timeout (ms)',
 *   defaultValue: 30000
 * },
 * {
 *   id: '{{NodeNameLower}}.theme',
 *   type: 'combo',
 *   name: 'Panel Theme',
 *   options: ['dark', 'light', 'system'],
 *   defaultValue: 'dark'
 * }
 */

import { app } from '/scripts/app.js';

/**
 * Register the {{NodeName}} settings extension with ComfyUI.
 *
 * The extension object must have:
 * - name: Unique extension identifier
 * - settings: Array of setting definitions
 */
app.registerExtension({
  /**
   * Extension name - must be unique across all ComfyUI extensions.
   * Convention: use lowercase with dots for namespacing.
   */
  name: '{{NodeNameLower}}.settings',

  /**
   * Settings Array
   *
   * Each setting will appear in the ComfyUI settings panel.
   * Add, remove, or modify settings as needed for your node.
   */
  settings: [
    {
      /**
       * Auto Show Panel Setting
       *
       * Controls whether the {{NodeName}} floating panel is automatically
       * displayed when ComfyUI loads.
       *
       * - id: Must be unique, conventionally prefixed with the extension name
       * - type: 'boolean' creates a checkbox/toggle
       * - name: Human-readable label shown in settings UI
       * - defaultValue: Initial value when setting hasn't been configured
       */
      id: '{{NodeNameLower}}.auto_show',
      type: 'boolean',
      name: 'Auto show {{NodeName}} panel',
      defaultValue: true
    },

    // =========================================================================
    // ADD YOUR CUSTOM SETTINGS BELOW
    // =========================================================================

    // Example: Uncomment and modify these to add more settings
    //
    // {
    //   id: '{{NodeNameLower}}.panel_opacity',
    //   type: 'slider',
    //   name: 'Panel Opacity',
    //   min: 0.3,
    //   max: 1.0,
    //   step: 0.1,
    //   defaultValue: 1.0
    // },
    //
    // {
    //   id: '{{NodeNameLower}}.debug_mode',
    //   type: 'boolean',
    //   name: 'Enable Debug Logging',
    //   defaultValue: false
    // },
  ],
});
