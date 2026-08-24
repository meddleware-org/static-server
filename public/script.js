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
