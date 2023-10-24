self.onnotificationclick = (event) => {
    event.notification.close(); // Close the notification when clicked

    console.log("On notification click: ", event.notification);
    const notificationData = event.notification.data;

    if (event.action === 'snooze') {
        if (notificationData && notificationData.taskId) {
            const taskId = notificationData.taskId;

            fetch(`/task/${taskId}/snooze`)
                .then(function (response) {
                    // Handle the response from the server
                })
                .catch(function (error) {
                    // Handle errors, e.g., failed network request
                    console.error(error);
                });
        }
    }

    event.waitUntil(
        clients
            .matchAll({
                type: "window",
                includeUncontrolled: true
            })
            .then((clientList) => {
                for (var i = 0; i < clientList.length; i++) {
                    var client = clientList[i];
                    if ('focus' in client) {
                        return client.focus();
                    }
                }
                if (clients.openWindow) {
                    return clients.openWindow('/');
                }
            }),
    );
};

self.addEventListener('install', function (event) {
    event.waitUntil(
        caches.open('my-cache').then(function (cache) {
            // Cache any assets or resources here.
        })
    );
});

self.addEventListener('fetch', function (event) {
    event.respondWith(
        caches.match(event.request).then(function (response) {
            return response || fetch(event.request);
        })
    );
});
