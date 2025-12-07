const CACHE_NAME = 'it-ops-v1';
const urlsToCache = [
  '/',
  '/auth/login',
  '/static/manifest.json',
  'https://cdn.tailwindcss.com',
  'https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css',
  'https://unpkg.com/htmx.org@1.9.5',
  'https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js'
];

self.addEventListener('install', event => {
  self.skipWaiting();
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(cache => {
        console.log('[SW] Opened cache');
        return cache.addAll(urlsToCache).catch(err => console.error('[SW] Cache error:', err));
      })
  );
});

self.addEventListener('activate', event => {
  event.waitUntil(clients.claim());
  console.log('[SW] Activated & Clients claimed');
});

self.addEventListener('fetch', event => {
  event.respondWith(
    fetch(event.request).catch(() => caches.match(event.request))
  );
});

// [FIX] Improved Push Listener
self.addEventListener('push', function(event) {
  console.log('[SW] Push Received:', event);

  let data = { title: 'New Notification', body: 'Check app for details.', url: '/' };

  if (event.data) {
    try {
      const json = event.data.json();
      data = { ...data, ...json }; // Merge default with incoming
    } catch (e) {
      console.warn('[SW] Push data is not JSON:', event.data.text());
      data.body = event.data.text();
    }
  }

  const options = {
    body: data.body,
    icon: 'https://cdn-icons-png.flaticon.com/512/906/906309.png',
    badge: 'https://cdn-icons-png.flaticon.com/512/906/906309.png',
    vibrate: [100, 50, 100],
    data: {
      url: data.url
    },
    // Menambahkan tag agar notifikasi tidak menumpuk jika topiknya sama
    tag: 'it-ops-notification' 
  };

  event.waitUntil(
    self.registration.showNotification(data.title, options)
  );
});

self.addEventListener('notificationclick', function(event) {
  console.log('[SW] Notification Clicked');
  event.notification.close();
  
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then(windowClients => {
      // Jika tab sudah terbuka, fokuskan
      for (let client of windowClients) {
        if (client.url === event.notification.data.url && 'focus' in client) {
          return client.focus();
        }
      }
      // Jika tidak, buka tab baru
      if (clients.openWindow) {
        return clients.openWindow(event.notification.data.url);
      }
    })
  );
});