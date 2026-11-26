/* static-server default page */

// Set <link rel="canonical"> from the current URL at runtime.
// The canonical href is always known client-side; setting it here ensures
// search engines receive the correct URL regardless of which host or port
// the page is served from.
(function () {
  // Build the tag by script so the canonical address is whatever host and port served this page.
  var link = document.createElement('link');
  link.rel = 'canonical';
  // Origin plus path, without the query string or fragment: those do not name the page.
  link.href = window.location.origin + window.location.pathname;
  document.head.appendChild(link);
})();

// Follow the visitor's light/dark preference (tokens.css themes on data-theme).
(function () {
  // matchMedia gives the live preference; the change listener below follows it while the page is open.
  var query = window.matchMedia('(prefers-color-scheme: dark)');
  /** apply writes the theme attribute that tokens.css switches on; the page never stores the choice (nothing is kept before an action). */
  function apply() {
    document.documentElement.setAttribute('data-theme', query.matches ? 'dark' : 'light');
  }
  // Set the theme once now, before first paint matters, then on every change of the system setting.
  apply();
  query.addEventListener('change', apply);
})();
