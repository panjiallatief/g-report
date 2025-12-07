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

// 1. Install & Force Active
self.addEventListener('install', event => {
  // [FIX] Paksa SW baru untuk langsung aktif (skip waiting phase)
  self.skipWaiting();
  
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(cache => {
        console.log('[SW] Opened cache');
        // Gunakan catch agar jika satu file gagal cache, install tetap jalan
        return cache.addAll(urlsToCache).catch(err => console.error('[SW] Cache error:', err));
      })
  );
});

// 2. Activate & Claim Clients
self.addEventListener('activate', event => {
  // [FIX] Langsung kontrol semua klien yang terbuka tanpa perlu reload
  event.waitUntil(clients.claim());
  console.log('[SW] Activated & Clients claimed');
});

// 3. Fetch Request
self.addEventListener('fetch', event => {
  // Strategi: Network First, Fallback Cache (Lebih aman untuk app dinamis)
  event.respondWith(
    fetch(event.request)
      .catch(() => {
        return caches.match(event.request);
      })
  );
});

// 4. Push Notification Listener
self.addEventListener('push', function(event) {
  if (event.data) {
    const data = event.data.json();
    
    const options = {
      body: data.body,
      icon: 'https://cdn-icons-png.flaticon.com/512/906/906309.png',
      badge: 'https://cdn-icons-png.flaticon.com/512/906/906309.png',
      vibrate: [100, 50, 100],
      data: {
        url: data.url || '/' 
      }
    };

    event.waitUntil(
      self.registration.showNotification(data.title, options)
    );
  }
});

// 5. Notification Click
self.addEventListener('notificationclick', function(event) {
  event.notification.close();
  event.waitUntil(
    clients.openWindow(event.notification.data.url)
  );
});