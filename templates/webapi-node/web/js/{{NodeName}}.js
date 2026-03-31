/**
 * {{NodeName}} - Floating Panel UI for ComfyUI
 *
 * This script creates a draggable floating panel that provides a user interface
 * for interacting with the {{NodeName}} backend API. It supports:
 * - Single topic generation via the /generate endpoint
 * - Batch processing of multiple topics via /batch_start, /batch_status, /batch_results
 *
 * The panel is positioned in the top-right corner of the ComfyUI interface and
 * can be dragged to any location by clicking and holding the title bar.
 *
 * Template Variables (replaced during node generation):
 * - {{NodeName}}: The display name of your node (e.g., "MyAwesomeNode")
 * - {{NodeNameLower}}: Lowercase version used for API routes (e.g., "myawesomenode")
 */

(function () {
  // ============================================================================
  // CONFIGURATION
  // ============================================================================

  /**
   * Namespace for API routes - all endpoints will be prefixed with this value
   * Example: If NS = 'mynode', routes become '/mynode/generate', '/mynode/batch_start', etc.
   */
  const NS = '{{NodeNameLower}}';

  // ============================================================================
  // SINGLETON GUARD
  // ============================================================================

  /**
   * Prevent multiple instances of the panel from being created.
   * If the panel already exists on the window object, exit early.
   * This handles cases where the script might be loaded multiple times.
   */
  if (window.__{{NodeNameLower}}_panel) {
    return;
  }

  // ============================================================================
  // DOM HELPER FUNCTION
  // ============================================================================

  /**
   * Create a DOM element with attributes and children.
   * This is a utility function to simplify element creation without jQuery.
   *
   * @param {string} tag - The HTML tag name (e.g., 'div', 'button', 'input')
   * @param {Object} attrs - Key-value pairs of attributes to set on the element
   * @param {Array} children - Array of child elements or strings to append
   * @returns {HTMLElement} The created DOM element
   *
   * @example
   * // Create a styled div with a text child
   * el('div', { class: 'container', id: 'main' }, ['Hello World'])
   *
   * @example
   * // Create nested elements
   * el('div', {}, [
   *   el('span', {}, ['Nested content']),
   *   el('button', { type: 'submit' }, ['Click me'])
   * ])
   */
  function el(tag, attrs = {}, children = []) {
    const element = document.createElement(tag);

    // Set all provided attributes on the element
    Object.entries(attrs).forEach(([key, value]) => {
      element.setAttribute(key, value);
    });

    // Append children - convert strings to text nodes, pass elements as-is
    children.forEach(child => {
      if (typeof child === 'string') {
        element.appendChild(document.createTextNode(child));
      } else {
        element.appendChild(child);
      }
    });

    return element;
  }

  // ============================================================================
  // PANEL CREATION
  // ============================================================================

  /**
   * Create the main floating panel container.
   * Store reference on window to enable singleton pattern and external access.
   */
  const panel = el('div');
  window.__{{NodeNameLower}}_panel = panel;

  /**
   * Panel Styling Configuration
   *
   * The panel uses inline styles for simplicity and to avoid CSS conflicts
   * with ComfyUI's existing stylesheets.
   *
   * Customize these values to match your preferred appearance:
   * - position: fixed - Keeps panel visible while scrolling
   * - top/right: Initial position in top-right corner
   * - z-index: 9999 - Ensures panel floats above most UI elements
   * - background: #111 - Dark background matching ComfyUI's dark theme
   * - color: #eee - Light text for contrast
   * - font: 12px sans-serif - Clean, readable font
   * - border-radius: 6px - Rounded corners for modern look
   * - min/max-width: Responsive sizing constraints
   */
  panel.style.cssText = `
    position: fixed;
    top: 60px;
    right: 20px;
    z-index: 9999;
    background: #111;
    color: #eee;
    font: 12px sans-serif;
    padding: 10px;
    border: 1px solid #444;
    border-radius: 6px;
    min-width: 240px;
    max-width: 320px;
  `;

  /**
   * Panel HTML Structure
   *
   * The panel contains the following sections:
   *
   * 1. HEADER (Draggable Title Bar)
   *    - Displays the node name
   *    - Can be clicked and dragged to reposition the panel
   *
   * 2. SINGLE GENERATION SECTION
   *    - Input field for entering a single topic
   *    - "Generate" button to trigger generation
   *
   * 3. OUTPUT DISPLAY
   *    - Pre-formatted area showing generation results
   *    - Scrollable for long outputs
   *
   * 4. BATCH PROCESSING SECTION
   *    - Textarea for comma-separated topic list
   *    - "Start Batch" button to begin batch processing
   *    - Status display for batch job progress
   *
   * Customize the HTML below to add/remove features or change the layout.
   */
  panel.innerHTML = `
    <!-- Draggable Header - Click and drag to move panel -->
    <div style="font-weight: bold; margin-bottom: 6px; cursor: move;">
      {{NodeName}} Panel
    </div>

    <!-- Single Topic Generation -->
    <div>
      <input
        id="sp_topic"
        placeholder="topic"
        style="width: 100%; margin-bottom: 4px;"
      />
      <button id="sp_go" style="width: 100%;">
        Generate
      </button>
    </div>

    <!-- Output Display Area -->
    <pre
      id="sp_out"
      style="margin-top: 6px; max-height: 140px; overflow: auto; background: #000; padding: 6px;"
    ></pre>

    <!-- Batch Processing Section -->
    <div style="margin-top: 8px; border-top: 1px solid #333; padding-top: 6px;">
      Batch (comma sep topics)<br/>
      <textarea
        id="sp_batch"
        style="width: 100%; height: 60px;"
      ></textarea>
      <button id="sp_batch_btn" style="width: 100%; margin-top: 4px;">
        Start Batch
      </button>
      <div id="sp_batch_status" style="margin-top: 4px;"></div>
    </div>
  `;

  // Add the panel to the document body
  document.body.appendChild(panel);

  // ============================================================================
  // DRAG FUNCTIONALITY
  // ============================================================================

  /**
   * Make the panel draggable by its header.
   *
   * This IIFE (Immediately Invoked Function Expression) sets up mouse event
   * listeners to enable drag-and-drop repositioning of the panel.
   *
   * How it works:
   * 1. mousedown on header: Record starting position and enable drag mode
   * 2. mousemove anywhere: If dragging, update panel position
   * 3. mouseup anywhere: Disable drag mode
   *
   * To change the drag handle, modify the 'head' variable to target a different element.
   */
  (function () {
    // Track the offset between mouse position and panel corner
    let offsetX, offsetY;

    // Flag indicating whether we're currently dragging
    let isDragging = false;

    // The draggable handle - first child is the title bar
    const head = panel.firstChild;

    // Start dragging when mouse is pressed on the header
    head.addEventListener('mousedown', (event) => {
      isDragging = true;

      // Calculate offset from mouse position to panel's top-left corner
      offsetX = event.clientX - panel.offsetLeft;
      offsetY = event.clientY - panel.offsetTop;
    });

    // Update panel position while dragging
    window.addEventListener('mousemove', (event) => {
      if (!isDragging) {
        return;
      }

      // Move panel to follow mouse, accounting for initial offset
      panel.style.left = (event.clientX - offsetX) + 'px';
      panel.style.top = (event.clientY - offsetY) + 'px';

      // Clear the 'right' property since we're now using 'left' for positioning
      panel.style.right = 'auto';
    });

    // Stop dragging when mouse is released anywhere
    window.addEventListener('mouseup', () => {
      isDragging = false;
    });
  })();

  // ============================================================================
  // API HELPER FUNCTIONS
  // ============================================================================

  /**
   * Parse JSON response and handle errors uniformly.
   *
   * This function:
   * 1. Attempts to parse the response as JSON
   * 2. Checks for HTTP errors (non-2xx status codes)
   * 3. Checks for application-level errors (success: false in response)
   * 4. Throws an error with a descriptive message if anything fails
   *
   * @param {Response} response - Fetch API Response object
   * @returns {Promise<Object>} Parsed JSON data
   * @throws {Error} If the response indicates an error
   *
   * @example
   * try {
   *   const data = await j(await fetch('/api/endpoint'));
   *   console.log(data);
   * } catch (error) {
   *   console.error('API error:', error.message);
   * }
   */
  async function j(response) {
    let jsonData = {};

    // Attempt to parse JSON - some errors may not return valid JSON
    try {
      jsonData = await response.json();
    } catch {
      // If JSON parsing fails, we'll rely on the HTTP status for error info
    }

    // Check for errors: HTTP error status OR explicit success: false in response
    if (!response.ok || jsonData.success === false) {
      throw new Error(jsonData.error || response.status);
    }

    return jsonData;
  }

  /**
   * Make a POST request to the API.
   *
   * @param {string} path - API endpoint path (without namespace prefix)
   * @param {Object} body - Request body to be JSON-serialized
   * @returns {Promise<Object>} Parsed JSON response
   * @throws {Error} If the request fails
   *
   * @example
   * // POST to /mynode/generate
   * const result = await post('/generate', { topic: 'AI assistants' });
   */
  async function post(path, body) {
    const url = '/' + NS + path;

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(body)
    });

    return j(response);
  }

  /**
   * Make a GET request to the API with optional query parameters.
   *
   * @param {string} path - API endpoint path (without namespace prefix)
   * @param {Object} queryParams - Key-value pairs to add as URL query parameters
   * @returns {Promise<Object>} Parsed JSON response
   * @throws {Error} If the request fails
   *
   * @example
   * // GET /mynode/batch_status?job_id=abc123
   * const status = await get('/batch_status', { job_id: 'abc123' });
   */
  async function get(path, queryParams = {}) {
    // Construct full URL with namespace prefix
    const url = new URL('/' + NS + path, location.origin);

    // Add query parameters to the URL
    Object.entries(queryParams).forEach(([key, value]) => {
      url.searchParams.set(key, value);
    });

    return j(await fetch(url));
  }

  // ============================================================================
  // EVENT HANDLERS
  // ============================================================================

  /**
   * Single Topic Generation Handler
   *
   * Triggered when the "Generate" button is clicked.
   *
   * Flow:
   * 1. Show loading indicator ("...")
   * 2. Send POST request to /generate with the topic
   * 3. Display the result in the output area
   * 4. If error occurs, display error message
   *
   * Customize the API endpoint or request format in the post() call below.
   */
  panel.querySelector('#sp_go').onclick = async () => {
    const outputArea = panel.querySelector('#sp_out');
    const topicInput = panel.querySelector('#sp_topic');

    // Show loading indicator
    outputArea.textContent = '...';

    try {
      // Send generation request to backend
      // Modify the endpoint path or body structure as needed for your API
      const response = await post('/generate', {
        topic: topicInput.value
      });

      // Display the result
      // Adjust 'response.result' to match your API's response structure
      outputArea.textContent = response.result;

    } catch (error) {
      // Display error message
      outputArea.textContent = 'ERR ' + error.message;
    }
  };

  /**
   * Batch Processing Handler
   *
   * Triggered when the "Start Batch" button is clicked.
   *
   * Flow:
   * 1. Parse comma-separated topics from textarea
   * 2. Start batch job via /batch_start endpoint
   * 3. Poll /batch_status every 1.2 seconds for progress updates
   * 4. When complete (status !== 'running'), fetch results from /batch_results
   * 5. Display all results in the output area
   *
   * Polling Configuration:
   * - POLL_INTERVAL: 1200ms (1.2 seconds) - Adjust for responsiveness vs server load
   *
   * Customize the API endpoints or polling logic as needed for your backend.
   */
  panel.querySelector('#sp_batch_btn').onclick = async () => {
    // Configuration
    const POLL_INTERVAL = 1200; // milliseconds between status checks

    // Get DOM references
    const batchTextarea = panel.querySelector('#sp_batch');
    const statusDisplay = panel.querySelector('#sp_batch_status');
    const outputArea = panel.querySelector('#sp_out');

    // Parse topics: split by comma, trim whitespace, remove empty entries
    const rawInput = batchTextarea.value;
    const topics = rawInput
      .split(',')
      .map(topic => topic.trim())
      .filter(topic => topic.length > 0);

    try {
      // Start the batch job
      // The backend should return a job_id for tracking
      const { job_id } = await post('/batch_start', { topics });

      statusDisplay.textContent = 'Job ' + job_id + ' started';

      // Set up polling interval to check job status
      const pollInterval = setInterval(async () => {
        try {
          // Check current job status
          const statusResponse = await get('/batch_status', { job_id });
          const jobStatus = statusResponse.data.status;
          const progress = statusResponse.data.progress;

          // Update status display with current progress
          statusDisplay.textContent = 'Status: ' + jobStatus + ' ' + progress + '%';

          // Check if job is complete (any status other than 'running')
          if (jobStatus !== 'running') {
            // Stop polling
            clearInterval(pollInterval);

            // Fetch final results
            const resultsResponse = await get('/batch_results', { job_id });

            // Display all results, one per line
            // Adjust 'resultsResponse.results' to match your API's response structure
            outputArea.textContent = resultsResponse.results.join('\n');
          }

        } catch (error) {
          // Stop polling on error
          clearInterval(pollInterval);
          statusDisplay.textContent = 'ERR ' + error.message;
        }

      }, POLL_INTERVAL);

    } catch (error) {
      statusDisplay.textContent = 'ERR ' + error.message;
    }
  };

})();
