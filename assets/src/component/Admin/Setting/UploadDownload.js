import Button from "@material-ui/core/Button";
import FormControl from "@material-ui/core/FormControl";
import FormControlLabel from "@material-ui/core/FormControlLabel";
import FormHelperText from "@material-ui/core/FormHelperText";
import Input from "@material-ui/core/Input";
import InputLabel from "@material-ui/core/InputLabel";
import { makeStyles } from "@material-ui/core/styles";
import Switch from "@material-ui/core/Switch";
import Typography from "@material-ui/core/Typography";
import React, { useCallback, useEffect, useState } from "react";
import { useDispatch } from "react-redux";
import { toggleSnackbar } from "../../../actions";
import API from "../../../middleware/Api";
import SizeInput from "../Common/SizeInput";

const useStyles = makeStyles((theme) => ({
    root: {
        [theme.breakpoints.up("md")]: {
            marginLeft: 100,
        },
        marginBottom: 40,
    },
    form: {
        maxWidth: 400,
        marginTop: 20,
        marginBottom: 20,
    },
    formContainer: {
        [theme.breakpoints.up("md")]: {
            padding: "0px 24px 0 24px",
        },
    },
}));

export default function UploadDownload() {
    const classes = useStyles();
    const [loading, setLoading] = useState(false);
    const [options, setOptions] = useState({
        max_worker_num: "1",
        max_parallel_transfer: "1",
        temp_path: "",
        maxEditSize: "0",
        onedrive_chunk_retries: "0",
        archive_timeout: "0",
        download_timeout: "0",
        preview_timeout: "0",
        doc_preview_timeout: "0",
        upload_credential_timeout: "0",
        upload_session_timeout: "0",
        slave_api_timeout: "0",
        onedrive_monitor_timeout: "0",
        share_download_session_timeout: "0",
        onedrive_callback_check: "0",
        reset_after_upload_failed: "0",
        onedrive_source_timeout: "0",
    });

    const handleCheckChange = (name) => (event) => {
        const value = event.target.checked ? "1" : "0";
        setOptions({
            ...options,
            [name]: value,
        });
    };

    const handleChange = (name) => (event) => {
        setOptions({
            ...options,
            [name]: event.target.value,
        });
    };

    const dispatch = useDispatch();
    const ToggleSnackbar = useCallback(
        (vertical, horizontal, msg, color) =>
            dispatch(toggleSnackbar(vertical, horizontal, msg, color)),
        [dispatch]
    );

    useEffect(() => {
        API.post("/admin/setting", {
            keys: Object.keys(options),
        })
            .then((response) => {
                setOptions(response.data);
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            });
        // eslint-disable-next-line
    }, []);

    const submit = (e) => {
        e.preventDefault();
        setLoading(true);
        const option = [];
        Object.keys(options).forEach((k) => {
            option.push({
                key: k,
                value: options[k],
            });
        });
        API.patch("/admin/setting", {
            options: option,
        })
            .then(() => {
                ToggleSnackbar("top", "right", "設定已更改", "success");
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            })
            .then(() => {
                setLoading(false);
            });
    };

    return (
        <div>
            <form onSubmit={submit}>
                <div className={classes.root}>
                    <Typography variant="h6" gutterBottom>
                        儲存與傳輸
                    </Typography>
                    <div className={classes.formContainer}>
                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    Worker 數量
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.max_worker_num}
                                    onChange={handleChange("max_worker_num")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    任務佇列最多並行執行的任務數，儲存後需要重啟
                                    Cloudreve 生效
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    中轉並行傳輸
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.max_parallel_transfer}
                                    onChange={handleChange(
                                        "max_parallel_transfer"
                                    )}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    任務佇列中轉任務傳輸時，最大並行協程數
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <InputLabel htmlFor="component-helper">
                                    暫存資料夾
                                </InputLabel>
                                <Input
                                    value={options.temp_path}
                                    onChange={handleChange("temp_path")}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    用於存放打包下載、解壓縮、壓縮等任務產生的暫存檔的目錄路徑
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <SizeInput
                                    value={options.maxEditSize}
                                    onChange={handleChange("maxEditSize")}
                                    required
                                    min={0}
                                    max={2147483647}
                                    label={"文字文件線上編輯大小"}
                                />
                                <FormHelperText id="component-helper-text">
                                    文字文件可線上編輯的最大大小，超出此大小的文件無法線上編輯
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    OneDrive 分片錯誤重試
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 0,
                                        step: 1,
                                    }}
                                    value={options.onedrive_chunk_retries}
                                    onChange={handleChange(
                                        "onedrive_chunk_retries"
                                    )}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    OneDrive
                                    儲存策略分片上傳失敗後重試的最大次數，只適用於服務端上傳或中轉
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                options.reset_after_upload_failed ===
                                                "1"
                                            }
                                            onChange={handleCheckChange(
                                                "reset_after_upload_failed"
                                            )}
                                        />
                                    }
                                    label="上傳校驗失敗時強制重設連接"
                                />
                                <FormHelperText id="component-helper-text">
                                    開啟後，如果本次策略、大頭貼等資料上傳校驗失敗，伺服器會強制重設連接
                                </FormHelperText>
                            </FormControl>
                        </div>
                    </div>
                </div>

                <div className={classes.root}>
                    <Typography variant="h6" gutterBottom>
                        有效期 (秒)
                    </Typography>

                    <div className={classes.formContainer}>
                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    打包下載
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.archive_timeout}
                                    onChange={handleChange("archive_timeout")}
                                    required
                                />
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    下載工作階段
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.download_timeout}
                                    onChange={handleChange("download_timeout")}
                                    required
                                />
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    預覽連結
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.preview_timeout}
                                    onChange={handleChange("preview_timeout")}
                                    required
                                />
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    Office 文件預覽連接
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.doc_preview_timeout}
                                    onChange={handleChange(
                                        "doc_preview_timeout"
                                    )}
                                    required
                                />
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    上傳憑證
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.upload_credential_timeout}
                                    onChange={handleChange(
                                        "upload_credential_timeout"
                                    )}
                                    required
                                />
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    上傳工作階段
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.upload_session_timeout}
                                    onChange={handleChange(
                                        "upload_session_timeout"
                                    )}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    超出後不再處理此上傳的回調請求
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    從機API請求
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.slave_api_timeout}
                                    onChange={handleChange("slave_api_timeout")}
                                    required
                                />
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    分享下載工作階段
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={
                                        options.share_download_session_timeout
                                    }
                                    onChange={handleChange(
                                        "share_download_session_timeout"
                                    )}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    設定時間內重複下載分享文件，不會被記入總下載次數
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    OneDrive 用戶端上傳監控間隔
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.onedrive_monitor_timeout}
                                    onChange={handleChange(
                                        "onedrive_monitor_timeout"
                                    )}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    每間隔所設定時間，Cloudreve 會向 OneDrive
                                    請求檢查用戶端上傳情況已確保用戶端上傳可控
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    OneDrive 回調等待
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        step: 1,
                                    }}
                                    value={options.onedrive_callback_check}
                                    onChange={handleChange(
                                        "onedrive_callback_check"
                                    )}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    OneDrive
                                    用戶端上傳完成後，等待回調的最大時間，如果超出會被認為上傳失敗
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    OneDrive 下載請求快取
                                </InputLabel>
                                <Input
                                    type={"number"}
                                    inputProps={{
                                        min: 1,
                                        max: 3659,
                                        step: 1,
                                    }}
                                    value={options.onedrive_source_timeout}
                                    onChange={handleChange(
                                        "onedrive_source_timeout"
                                    )}
                                    required
                                />
                                <FormHelperText id="component-helper-text">
                                    OneDrive 獲取文件下載 URL
                                    後可將結果快取，減輕熱門文件下載API請求頻率
                                </FormHelperText>
                            </FormControl>
                        </div>
                    </div>
                </div>

                <div className={classes.root}>
                    <Button
                        disabled={loading}
                        type={"submit"}
                        variant={"contained"}
                        color={"primary"}
                    >
                        儲存
                    </Button>
                </div>
            </form>
        </div>
    );
}
