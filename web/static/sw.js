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

// 1. Install Service Worker & Cache Aset Statis
self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(cache => {
        console.log('Opened cache');
        return cache.addAll(urlsToCache);
      })
  );
});

// 2. Fetch Request (Network First, Fallback to Cache)
self.addEventListener('fetch', event => {
  event.respondWith(
    fetch(event.request).catch(() => {
      return caches.match(event.request);
    })
  );
});

// 3. Listener untuk Push Notification (Persiapan Fitur Alert)
self.addEventListener('push', function(event) {
  if (event.data) {
    const data = event.data.json();
    
    const options = {
      body: data.body,
      icon: 'https://cdn-icons-png.flaticon.com/512/906/906309.png', // Ganti dengan icon lokal nanti
      badge: 'https://cdn-icons-png.flaticon.com/512/906/906309.png',
      vibrate: [100, 50, 100], // Getar: Bzz-bz-Bzz (Pola Alert)
      data: {
        url: data.url || '/' 
      }
    };

    event.waitUntil(
      self.registration.showNotification(data.title, options)
    );
  }
});

// 4. Handle Klik Notifikasi
self.addEventListener('notificationclick', function(event) {
  event.notification.close();
  event.waitUntil(
    clients.openWindow(event.notification.data.url)
  );
});