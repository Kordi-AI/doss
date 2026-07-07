(function () {
  var ACTION_ATTR = "data-doss-root-markdown-action";
  var SEPARATOR_ATTR = "data-doss-root-markdown-separator";
  var LABEL = "View as Markdown";

  function removeInjectedAction() {
    var action = document.querySelector("[" + ACTION_ATTR + "]");
    var separator = document.querySelector("[" + SEPARATOR_ATTR + "]");
    if (action) action.remove();
    if (separator) separator.remove();
  }

  function syncRootMarkdownAction() {
    var actions = document.querySelector(".fern-page-actions");
    if (!actions) return;

    if (window.location.pathname !== "/") {
      removeInjectedAction();
      return;
    }

    if (actions.querySelector("[" + ACTION_ATTR + "]")) return;
    if (actions.textContent && actions.textContent.indexOf(LABEL) !== -1) return;

    var link = document.createElement("a");
    link.href = "/.md";
    link.title = "View this page as Markdown";
    link.textContent = LABEL;
    link.setAttribute(ACTION_ATTR, "true");
    link.className =
      "px-2 py-1 rounded-2 text-(color:--grayscale-a11) whitespace-nowrap hover:bg-(color:--accent-a3) hover:text-(color:--accent-12) transition-colors flex items-center gap-1.5 cursor-pointer";

    var separator = document.createElement("span");
    separator.setAttribute(SEPARATOR_ATTR, "true");
    separator.setAttribute("aria-hidden", "true");
    separator.className = "text-(color:--grayscale-a8)";
    separator.style.margin = "0 8px";
    separator.textContent = "|";

    var first = actions.firstChild;
    actions.insertBefore(link, first);
    actions.insertBefore(separator, first);
  }

  function install() {
    syncRootMarkdownAction();

    var observer = new MutationObserver(syncRootMarkdownAction);
    observer.observe(document.documentElement, {
      childList: true,
      subtree: true
    });

    window.addEventListener("popstate", syncRootMarkdownAction);
    ["pushState", "replaceState"].forEach(function (name) {
      var original = history[name];
      history[name] = function () {
        var result = original.apply(this, arguments);
        setTimeout(syncRootMarkdownAction, 0);
        return result;
      };
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", install);
  } else {
    install();
  }
})();
