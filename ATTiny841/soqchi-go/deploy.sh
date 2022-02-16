#!/bin/sh

export GOOGLE_APPLICATION_CREDENTIALS=iot-oisw-316feba13ec0.json

gcloud functions deploy PlainTelegramMessage  \
  --env-vars-file gcf_soqchi.env.yaml --region europe-west3 --runtime go116 --trigger-topic PlainMessage2Telegram

gcloud functions deploy Watchdog  \
  --env-vars-file gcf_soqchi.env.yaml --region europe-west3 --runtime go116 --trigger-topic watchdog

gcloud functions deploy Device \
  --env-vars-file gcf_soqchi.env.yaml --region europe-west3 --runtime go116 --trigger-http --allow-unauthenticated

gcloud functions deploy TelegramHTTPReceiver  \
   --env-vars-file gcf_soqchi.env.yaml --region europe-west3 --runtime go116 --trigger-http --allow-unauthenticated
