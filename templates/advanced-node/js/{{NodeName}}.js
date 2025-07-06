/**
 * {{NodeName}} - Advanced ComfyUI Web Components
 * 
 * This file provides advanced web UI components for the {{NodeName}} node,
 * including real-time controls, preview panels, and settings management.
 * 
 * Author: {{Author}}
 */

import { app } from "/scripts/app.js";
import { ComfyWidgets } from "/scripts/widgets.js";
import { api } from "/scripts/api.js";

// {{NodeName}} UI Controller
class {{NodeName}}UIController {
    constructor(node) {
        this.node = node;
        this.previewCanvas = null;
        this.settingsPanel = null;
        this.isPreviewEnabled = true;
        this.settings = this.loadSettings();
        
        this.setupAdvancedUI();
        this.setupEventListeners();
    }

    /**
     * Setup advanced UI components
     */
    setupAdvancedUI() {
        // Create main container
        const container = document.createElement("div");
        container.className = "{{NodeNameLower}}-container";
        container.style.cssText = `
            background: #2a2a2a;
            border: 1px solid #444;
            border-radius: 8px;
            padding: 12px;
            margin: 8px 0;
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
        `;

        // Create header
        const header = this.createHeader();
        container.appendChild(header);

        // Create preview panel
        if (this.isPreviewEnabled) {
            const previewPanel = this.createPreviewPanel();
            container.appendChild(previewPanel);
        }

        // Create settings panel
        const settingsPanel = this.createSettingsPanel();
        container.appendChild(settingsPanel);

        // Create status bar
        const statusBar = this.createStatusBar();
        container.appendChild(statusBar);

        // Add to node
        this.node.addDOMWidget("{{NodeNameLower}}_ui", "div", container);
    }

    /**
     * Create header with controls
     */
    createHeader() {
        const header = document.createElement("div");
        header.className = "{{NodeNameLower}}-header";
        header.style.cssText = `
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 12px;
            padding-bottom: 8px;
            border-bottom: 1px solid #555;
        `;

        // Title
        const title = document.createElement("h3");
        title.textContent = "{{NodeName}} Controls";
        title.style.cssText = `
            margin: 0;
            color: #ffffff;
            font-size: 14px;
            font-weight: 600;
        `;

        // Control buttons
        const controls = document.createElement("div");
        controls.style.cssText = "display: flex; gap: 8px;";

        // Preview toggle
        const previewToggle = this.createButton("👁", "Toggle Preview", () => {
            this.togglePreview();
        });

        // Settings toggle
        const settingsToggle = this.createButton("⚙", "Settings", () => {
            this.toggleSettings();
        });

        // Reset button
        const resetButton = this.createButton("🔄", "Reset", () => {
            this.resetSettings();
        });

        controls.appendChild(previewToggle);
        controls.appendChild(settingsToggle);
        controls.appendChild(resetButton);

        header.appendChild(title);
        header.appendChild(controls);

        return header;
    }

    /**
     * Create preview panel
     */
    createPreviewPanel() {
        const panel = document.createElement("div");
        panel.className = "{{NodeNameLower}}-preview-panel";
        panel.style.cssText = `
            background: #1a1a1a;
            border: 1px solid #333;
            border-radius: 6px;
            padding: 8px;
            margin-bottom: 12px;
            min-height: 200px;
            display: flex;
            align-items: center;
            justify-content: center;
        `;

        // Create canvas for preview
        this.previewCanvas = document.createElement("canvas");
        this.previewCanvas.width = 256;
        this.previewCanvas.height = 256;
        this.previewCanvas.style.cssText = `
            max-width: 100%;
            max-height: 100%;
            border: 1px solid #444;
            background: #000;
        `;

        // Preview label
        const label = document.createElement("div");
        label.textContent = "Preview will appear here";
        label.style.cssText = `
            color: #888;
            font-size: 12px;
            text-align: center;
        `;

        panel.appendChild(this.previewCanvas);
        panel.appendChild(label);

        return panel;
    }

    /**
     * Create settings panel
     */
    createSettingsPanel() {
        this.settingsPanel = document.createElement("div");
        this.settingsPanel.className = "{{NodeNameLower}}-settings-panel";
        this.settingsPanel.style.cssText = `
            background: #1e1e1e;
            border: 1px solid #333;
            border-radius: 6px;
            padding: 12px;
            margin-bottom: 12px;
            display: none;
        `;

        // Settings header
        const header = document.createElement("h4");
        header.textContent = "Advanced Settings";
        header.style.cssText = `
            margin: 0 0 12px 0;
            color: #ffffff;
            font-size: 13px;
            font-weight: 600;
        `;

        // Settings form
        const form = this.createSettingsForm();

        this.settingsPanel.appendChild(header);
        this.settingsPanel.appendChild(form);

        return this.settingsPanel;
    }

    /**
     * Create settings form
     */
    createSettingsForm() {
        const form = document.createElement("div");
        form.style.cssText = "display: grid; gap: 12px;";

        // Example settings
        const settings = [
            {
                key: "blur_radius",
                label: "Blur Radius",
                type: "range",
                min: 0,
                max: 10,
                step: 0.5,
                default: 1.0
            },
            {
                key: "sharpen_factor",
                label: "Sharpen Factor",
                type: "range",
                min: 0,
                max: 2,
                step: 0.1,
                default: 0.5
            },
            {
                key: "color_enhancement",
                label: "Color Enhancement",
                type: "checkbox",
                default: true
            },
            {
                key: "noise_reduction",
                label: "Noise Reduction",
                type: "select",
                options: ["none", "light", "medium", "heavy"],
                default: "light"
            }
        ];

        settings.forEach(setting => {
            const field = this.createSettingField(setting);
            form.appendChild(field);
        });

        return form;
    }

    /**
     * Create individual setting field
     */
    createSettingField(setting) {
        const field = document.createElement("div");
        field.style.cssText = "display: flex; flex-direction: column; gap: 4px;";

        // Label
        const label = document.createElement("label");
        label.textContent = setting.label;
        label.style.cssText = `
            color: #cccccc;
            font-size: 12px;
            font-weight: 500;
        `;

        // Input
        let input;
        switch (setting.type) {
            case "range":
                input = document.createElement("input");
                input.type = "range";
                input.min = setting.min;
                input.max = setting.max;
                input.step = setting.step;
                input.value = this.settings[setting.key] || setting.default;
                break;

            case "checkbox":
                input = document.createElement("input");
                input.type = "checkbox";
                input.checked = this.settings[setting.key] !== undefined 
                    ? this.settings[setting.key] 
                    : setting.default;
                break;

            case "select":
                input = document.createElement("select");
                setting.options.forEach(option => {
                    const optElement = document.createElement("option");
                    optElement.value = option;
                    optElement.textContent = option;
                    input.appendChild(optElement);
                });
                input.value = this.settings[setting.key] || setting.default;
                break;
        }

        input.style.cssText = `
            background: #333;
            border: 1px solid #555;
            color: #fff;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
        `;

        // Value display for ranges
        let valueDisplay;
        if (setting.type === "range") {
            valueDisplay = document.createElement("span");
            valueDisplay.textContent = input.value;
            valueDisplay.style.cssText = `
                color: #888;
                font-size: 11px;
                text-align: right;
            `;

            input.addEventListener("input", () => {
                valueDisplay.textContent = input.value;
                this.updateSetting(setting.key, parseFloat(input.value));
            });
        } else {
            input.addEventListener("change", () => {
                const value = setting.type === "checkbox" ? input.checked : input.value;
                this.updateSetting(setting.key, value);
            });
        }

        field.appendChild(label);
        field.appendChild(input);
        if (valueDisplay) field.appendChild(valueDisplay);

        return field;
    }

    /**
     * Create status bar
     */
    createStatusBar() {
        const statusBar = document.createElement("div");
        statusBar.className = "{{NodeNameLower}}-status-bar";
        statusBar.style.cssText = `
            background: #1a1a1a;
            border: 1px solid #333;
            border-radius: 4px;
            padding: 6px 10px;
            font-size: 11px;
            color: #888;
            display: flex;
            justify-content: space-between;
            align-items: center;
        `;

        const status = document.createElement("span");
        status.textContent = "Ready";
        status.id = "{{NodeNameLower}}-status";

        const info = document.createElement("span");
        info.textContent = "{{NodeName}} v{{Version}}";

        statusBar.appendChild(status);
        statusBar.appendChild(info);

        return statusBar;
    }

    /**
     * Create utility button
     */
    createButton(text, tooltip, onclick) {
        const button = document.createElement("button");
        button.textContent = text;
        button.title = tooltip;
        button.style.cssText = `
            background: #444;
            border: 1px solid #666;
            border-radius: 4px;
            color: #fff;
            padding: 4px 8px;
            font-size: 12px;
            cursor: pointer;
            transition: background 0.2s;
        `;

        button.addEventListener("mouseenter", () => {
            button.style.background = "#555";
        });

        button.addEventListener("mouseleave", () => {
            button.style.background = "#444";
        });

        button.addEventListener("click", onclick);

        return button;
    }

    /**
     * Setup event listeners
     */
    setupEventListeners() {
        // Listen for node execution
        api.addEventListener("executing", (event) => {
            if (event.detail?.node === this.node.id) {
                this.updateStatus("Processing...");
            }
        });

        api.addEventListener("executed", (event) => {
            if (event.detail?.node === this.node.id) {
                this.updateStatus("Complete");
                this.updatePreview(event.detail?.output);
            }
        });
    }

    /**
     * Toggle preview panel
     */
    togglePreview() {
        const panel = document.querySelector(".{{NodeNameLower}}-preview-panel");
        if (panel) {
            panel.style.display = panel.style.display === "none" ? "flex" : "none";
            this.isPreviewEnabled = panel.style.display !== "none";
        }
    }

    /**
     * Toggle settings panel
     */
    toggleSettings() {
        if (this.settingsPanel) {
            const isVisible = this.settingsPanel.style.display !== "none";
            this.settingsPanel.style.display = isVisible ? "none" : "block";
        }
    }

    /**
     * Reset settings to defaults
     */
    resetSettings() {
        this.settings = {};
        this.saveSettings();
        
        // Update UI
        const inputs = this.settingsPanel.querySelectorAll("input, select");
        inputs.forEach(input => {
            if (input.type === "range") {
                input.value = input.getAttribute("data-default") || "0";
            } else if (input.type === "checkbox") {
                input.checked = false;
            } else {
                input.value = "";
            }
        });

        this.updateStatus("Settings reset");
    }

    /**
     * Update individual setting
     */
    updateSetting(key, value) {
        this.settings[key] = value;
        this.saveSettings();
        
        // Update node widget if exists
        const widget = this.node.widgets?.find(w => w.name === "settings_json");
        if (widget) {
            widget.value = JSON.stringify(this.settings, null, 2);
        }
    }

    /**
     * Update status display
     */
    updateStatus(message) {
        const status = document.getElementById("{{NodeNameLower}}-status");
        if (status) {
            status.textContent = message;
        }
    }

    /**
     * Update preview display
     */
    updatePreview(output) {
        if (!this.previewCanvas || !this.isPreviewEnabled) return;

        // This would be implemented based on ComfyUI's preview system
        // Placeholder for now
        const ctx = this.previewCanvas.getContext("2d");
        ctx.fillStyle = "#333";
        ctx.fillRect(0, 0, this.previewCanvas.width, this.previewCanvas.height);
        
        ctx.fillStyle = "#fff";
        ctx.font = "12px Arial";
        ctx.textAlign = "center";
        ctx.fillText("Preview Updated", this.previewCanvas.width / 2, this.previewCanvas.height / 2);
    }

    /**
     * Load settings from localStorage
     */
    loadSettings() {
        try {
            const saved = localStorage.getItem("{{NodeNameLower}}_settings");
            return saved ? JSON.parse(saved) : {};
        } catch (e) {
            return {};
        }
    }

    /**
     * Save settings to localStorage
     */
    saveSettings() {
        try {
            localStorage.setItem("{{NodeNameLower}}_settings", JSON.stringify(this.settings));
        } catch (e) {
            console.warn("Failed to save {{NodeNameLower}} settings:", e);
        }
    }
}

// Register the extension
app.registerExtension({
    name: "{{NodeName}}.AdvancedUI",
    async beforeRegisterNodeDef(nodeType, nodeData, app) {
        if (nodeData.name === "{{NodeName}}") {
            const onNodeCreated = nodeType.prototype.onNodeCreated;
            nodeType.prototype.onNodeCreated = function () {
                const result = onNodeCreated?.apply(this, arguments);
                
                // Initialize the UI controller
                this.{{NodeNameLower}}_ui_controller = new {{NodeName}}UIController(this);
                
                return result;
            };
        }
    }
});