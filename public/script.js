/* static-server default page */

// Set <link rel="canonical"> from the current URL at runtime.
// The canonical href is always known client-side; setting it here ensures
// search engines receive the correct URL regardless of which host or port
// the page is served from.
(function () {
  var link = document.createElement('link');
  link.rel = 'canonical';
  link.href = window.location.origin + window.location.pathname;
  document.head.appendChild(link);
})();

// Follow the visitor's light/dark preference (tokens.css themes on data-theme).
(function () {
  var query = window.matchMedia('(prefers-color-scheme: dark)');
  function apply() {
    document.documentElement.setAttribute('data-theme', query.matches ? 'dark' : 'light');
  }
  apply();
  query.addEventListener('change', apply);
})();
