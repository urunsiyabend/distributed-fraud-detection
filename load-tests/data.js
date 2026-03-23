import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const currencies = ['USD', 'EUR', 'GBP', 'JPY', 'CAD'];
const paymentMethods = ['card', 'wire', 'crypto'];

// %70 approved (low amount, known device)
// %20 review   (medium amount, unknown device)
// %10 blocked  (very high amount)
export function generateTransaction() {
    const roll = Math.random() * 100;
    let amount, deviceId, senderIdx;

    if (roll < 70) {
        // approved path
        amount = randomIntBetween(10, 500);
        senderIdx = randomIntBetween(1, 100);
        deviceId = `known-device-${senderIdx}`;
    } else if (roll < 90) {
        // review path — unknown device
        amount = randomIntBetween(500, 2000);
        senderIdx = randomIntBetween(1, 100);
        deviceId = `unknown-${Date.now()}-${randomIntBetween(1, 99999)}`;
    } else {
        // blocked path — critical amount (>3x threshold)
        amount = randomIntBetween(5000, 50000);
        senderIdx = randomIntBetween(1, 100);
        deviceId = `device-${randomIntBetween(1, 50)}`;
    }

    const senderId = `user-${senderIdx}`;
    let receiverIdx = randomIntBetween(1, 100);
    while (receiverIdx === senderIdx) {
        receiverIdx = randomIntBetween(1, 100);
    }

    return {
        id: `tx-${Date.now()}-${randomIntBetween(1, 999999)}`,
        amount: amount,
        currency: currencies[randomIntBetween(0, currencies.length - 1)],
        sender_id: senderId,
        receiver_id: `user-${receiverIdx}`,
        device_id: deviceId,
        ip: `${randomIntBetween(1, 254)}.${randomIntBetween(0, 255)}.${randomIntBetween(0, 255)}.${randomIntBetween(1, 254)}`,
        location: {
            lat: randomIntBetween(-90, 90) + Math.random(),
            lng: randomIntBetween(-180, 180) + Math.random(),
        },
        timestamp: new Date().toISOString(),
        payment_method: paymentMethods[randomIntBetween(0, paymentMethods.length - 1)],
    };
}
