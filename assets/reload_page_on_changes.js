/*!
 * Reload page on changes module
 *
 * @author   Gregor Noczinski
 * @license  MIT
 */

/**
 * Will automatically reload the page of the provided websocket endpoint
 * will send a corresponding message.
 *
 * In your base HTML file include the following snipped:
 * <script>CHANGE_STATUS_DETECTION_PATH="/path_to_your_endpoint";
 *      CHANGE_STATUS_DETECTION_SECRET="optional_only_if_required";</script>
 * <script src="path_to_this_script"></script>
 */

(function () {
    if (!window.CHANGE_STATUS_DETECTION_PATH) {
        throw new Error("There was CHANGE_STATUS_DETECTION_PATH variable no defined before this script was loaded.");
    }

    const path = window.CHANGE_STATUS_DETECTION_PATH;
    const secret = window.CHANGE_STATUS_DETECTION_SECRET;
    var ws = null;

    function websocketUrl() {
        var result = location.protocol.replace(/^http/, "ws") + "//";
        if (secret) {
            result += secret + "@";
        }
        result += location.host + path;
        return result;
    }

    function onMessage(event) {
        const source = ws && ws.url ? ws.url : "unknown";
        if (event.type === "message") {
            var message;
            try {
                message = eval("x = " + event.data);
                if (typeof message !== "object") {
                    throw new Error("Not an object");
                }
            } catch (e) {
                console.warn("Got unexpected message from " + source + ".", e, event);
                return;
            }
            if (message.active === false) {
                console.info("Got message from " + source + " but the server is currently not ready. Ignore this message for now.");
                return;
            }
            console.info("Force reload page.");
            location.reload(true);
        }
    }

    function onClose(event) {
        console.warn("Remote was closed unexpectedly. Retry open it again in 5s...", event);
        setTimeout(function () {
            initiate();
        }, 5000);
    }

    function initiate() {
        const url = websocketUrl();
        console.log("Going to listen for changes of that page at '" + url + "'...");
        ws = new WebSocket(url);
        ws.onmessage = onMessage;
        ws.onclose = onClose;
    }

    initiate();
}());
