#!/bin/sh

export GOOGLE_APPLICATION_CREDENTIALS=iot-oisw-e1a229224500.json

gcloud functions deploy PlainTelegramMessage  \
  --env-vars-file gcf_soqchi.env.yaml --region europe-west3 --runtime go113 --trigger-topic PlainMessage2Telegram

gcloud functions deploy Watchdog  \
  --env-vars-file gcf_soqchi.env.yaml --region europe-west3 --runtime go113 --trigger-topic watchdog

gcloud functions deploy Device \
  --env-vars-file gcf_soqchi.env.yaml --region europe-west3 --runtime go113 --trigger-http --allow-unauthenticated

gcloud functions deploy TelegramHTTPReceiver  \
   --env-vars-file gcf_soqchi.env.yaml --region europe-west3 --runtime go113 --trigger-http --allow-unauthenticated