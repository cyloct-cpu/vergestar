import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";
import { runLocalRuntimeBootstrap } from "@/services/local-runtime-bootstrap";
import { bootstrapAppearance } from "@/services/appearance-bootstrap";

runLocalRuntimeBootstrap(
    {
        get href() {
            return window.location.href;
        },
        replaceUrl(url) {
            window.history.replaceState(window.history.state, "", url);
        },
        removeStorageItem(key) {
            window.localStorage.removeItem(key);
        },
    },
    () => {
        // vergestar: 暂不开放 /welcome 品牌首页，统一走工作台应用；恢复时改回按路径分支加载 welcome-application。
        void bootstrapAppearance().finally(() => import("./application"));
    },
);
