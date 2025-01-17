import Button from "@material-ui/core/Button";
import FormControl from "@material-ui/core/FormControl";
import FormControlLabel from "@material-ui/core/FormControlLabel";
import FormHelperText from "@material-ui/core/FormHelperText";
import InputLabel from "@material-ui/core/InputLabel";
import MenuItem from "@material-ui/core/MenuItem";
import Select from "@material-ui/core/Select";
import { makeStyles } from "@material-ui/core/styles";
import Switch from "@material-ui/core/Switch";
import Typography from "@material-ui/core/Typography";
import React, { useCallback, useEffect, useState } from "react";
import { useDispatch } from "react-redux";
import { toggleSnackbar } from "../../../actions";
import API from "../../../middleware/Api";
import AlertDialog from "../Dialogs/Alert";

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

export default function Access() {
    const classes = useStyles();
    const [loading, setLoading] = useState(false);
    const [options, setOptions] = useState({
        register_enabled: "1",
        default_group: "1",
        email_active: "0",
        login_captcha: "0",
        reg_captcha: "0",
        forget_captcha: "0",
        authn_enabled: "0",
    });
    const [siteURL, setSiteURL] = useState("");
    const [groups, setGroups] = useState([]);
    const [httpAlert, setHttpAlert] = useState(false);

    const handleChange = (name) => (event) => {
        let value = event.target.value;
        if (event.target.checked !== undefined) {
            value = event.target.checked ? "1" : "0";
        }
        setOptions({
            ...options,
            [name]: value,
        });
    };

    const handleInputChange = (name) => (event) => {
        const value = event.target.value;
        setOptions({
            ...options,
            [name]: value,
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
            keys: [...Object.keys(options), "siteURL"],
        })
            .then((response) => {
                setSiteURL(response.data.siteURL);
                delete response.data.siteURL;
                setOptions(response.data);
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            });

        API.get("/admin/groups")
            .then((response) => {
                setGroups(response.data);
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
            <AlertDialog
                title={"提示"}
                msg={
                    "Web Authn 需要您的站點啟用 HTTPS，並確認 參數設定 - 站點訊息 - 站點URL 也使用了 HTTPS 後才能開啟。"
                }
                onClose={() => setHttpAlert(false)}
                open={httpAlert}
            />
            <form onSubmit={submit}>
                <div className={classes.root}>
                    <Typography variant="h6" gutterBottom>
                        註冊與登入
                    </Typography>
                    <div className={classes.formContainer}>
                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                options.register_enabled === "1"
                                            }
                                            onChange={handleChange(
                                                "register_enabled"
                                            )}
                                        />
                                    }
                                    label="允許新使用者註冊"
                                />
                                <FormHelperText id="component-helper-text">
                                    關閉後，無法再通過前台註冊新的使用者
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                options.email_active === "1"
                                            }
                                            onChange={handleChange(
                                                "email_active"
                                            )}
                                        />
                                    }
                                    label="郵件啟動"
                                />
                                <FormHelperText id="component-helper-text">
                                    開啟後，新使用者註冊需要點擊郵件中的啟動連結才能完成。請確認郵件發送設定是否正確，否則啟動郵件無法送達
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                options.reg_captcha === "1"
                                            }
                                            onChange={handleChange(
                                                "reg_captcha"
                                            )}
                                        />
                                    }
                                    label="註冊驗證碼"
                                />
                                <FormHelperText id="component-helper-text">
                                    是否啟用註冊表單驗證碼
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                options.login_captcha === "1"
                                            }
                                            onChange={handleChange(
                                                "login_captcha"
                                            )}
                                        />
                                    }
                                    label="登入驗證碼"
                                />
                                <FormHelperText id="component-helper-text">
                                    是否啟用登入表單驗證碼
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                options.forget_captcha === "1"
                                            }
                                            onChange={handleChange(
                                                "forget_captcha"
                                            )}
                                        />
                                    }
                                    label="找回密碼驗證碼"
                                />
                                <FormHelperText id="component-helper-text">
                                    是否啟用找回密碼錶單驗證碼
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl fullWidth>
                                <FormControlLabel
                                    control={
                                        <Switch
                                            checked={
                                                options.authn_enabled === "1"
                                            }
                                            onChange={(e) => {
                                                if (
                                                    !siteURL.startsWith(
                                                        "https://"
                                                    )
                                                ) {
                                                    setHttpAlert(true);
                                                    return;
                                                }
                                                handleChange("authn_enabled")(
                                                    e
                                                );
                                            }}
                                        />
                                    }
                                    label="Web Authn"
                                />
                                <FormHelperText id="component-helper-text">
                                    是否允許使用者使用綁定的外部驗證器登入，站點必須啟動
                                    HTTPS 才能使用。
                                </FormHelperText>
                            </FormControl>
                        </div>

                        <div className={classes.form}>
                            <FormControl>
                                <InputLabel htmlFor="component-helper">
                                    預設使用者群組
                                </InputLabel>
                                <Select
                                    value={options.default_group}
                                    onChange={handleInputChange(
                                        "default_group"
                                    )}
                                    required
                                >
                                    {groups.map((v) => {
                                        if (v.ID === 3) {
                                            return null;
                                        }
                                        return (
                                            <MenuItem
                                                key={v.ID}
                                                value={v.ID.toString()}
                                            >
                                                {v.Name}
                                            </MenuItem>
                                        );
                                    })}
                                </Select>
                                <FormHelperText id="component-helper-text">
                                    使用者註冊後的初始使用者群組
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
