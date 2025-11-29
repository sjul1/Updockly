import { createApp } from "vue";
import Toast, { type PluginOptions, POSITION } from "vue-toastification";
import "vue-toastification/dist/index.css";
import "./style.css";
import App from "./App.vue";

const options: PluginOptions = {
  position: POSITION.TOP_RIGHT,
  timeout: 3000,
  closeOnClick: true,
  pauseOnHover: true,
  draggable: true,
};

const app = createApp(App);
app.use(Toast, options);
app.mount("#app");
