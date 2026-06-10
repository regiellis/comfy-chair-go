import { app } from "../../scripts/app.js";

// Frontend extension for {{NodeName}}. The frontend API is the same for V1
// and V3 nodes; match nodes by their node_id (nodeData.name / comfyClass).
app.registerExtension({
    name: "{{NodeNameLower}}",

    async setup() {
        console.log("[{{NodeName}}] Extension loaded.");
    },

    async beforeRegisterNodeDef(nodeType, nodeData, app) {
        if (nodeData.name !== "{{NodeName}}") return;

        // Example: run code when an instance of this node is created.
        const onNodeCreated = nodeType.prototype.onNodeCreated;
        nodeType.prototype.onNodeCreated = function () {
            const ret = onNodeCreated ? onNodeCreated.apply(this, arguments) : undefined;
            // Customize the node instance here (extra widgets, styling, handlers).
            return ret;
        };
    },

    async nodeCreated(node) {
        if (node?.comfyClass !== "{{NodeName}}") return;
        // Alternative hook: runs for every created instance of this node.
    },
});
