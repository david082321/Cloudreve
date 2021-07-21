import Button from "@material-ui/core/Button";
import FormControl from "@material-ui/core/FormControl";
import FormControlLabel from "@material-ui/core/FormControlLabel";
import Input from "@material-ui/core/Input";
import Radio from "@material-ui/core/Radio";
import RadioGroup from "@material-ui/core/RadioGroup";
import Table from "@material-ui/core/Table";
import TableBody from "@material-ui/core/TableBody";
import TableCell from "@material-ui/core/TableCell";
import TableContainer from "@material-ui/core/TableContainer";
import TableHead from "@material-ui/core/TableHead";
import TableRow from "@material-ui/core/TableRow";
import Typography from "@material-ui/core/Typography";
import React, { useCallback, useState } from "react";
import { useDispatch } from "react-redux";
import { toggleSnackbar } from "../../../../actions";
import API from "../../../../middleware/Api";

export default function EditPro(props) {
    const [, setLoading] = useState(false);
    const [policy, setPolicy] = useState(props.policy);

    const handleChange = (name) => (event) => {
        setPolicy({
            ...policy,
            [name]: event.target.value,
        });
    };

    const handleOptionChange = (name) => (event) => {
        setPolicy({
            ...policy,
            OptionsSerialized: {
                ...policy.OptionsSerialized,
                [name]: event.target.value,
            },
        });
    };

    const dispatch = useDispatch();
    const ToggleSnackbar = useCallback(
        (vertical, horizontal, msg, color) =>
            dispatch(toggleSnackbar(vertical, horizontal, msg, color)),
        [dispatch]
    );

    const submitPolicy = (e) => {
        e.preventDefault();
        setLoading(true);

        const policyCopy = { ...policy };
        policyCopy.OptionsSerialized = { ...policyCopy.OptionsSerialized };

        // 類型轉換
        policyCopy.AutoRename = policyCopy.AutoRename === "true";
        policyCopy.IsPrivate = policyCopy.IsPrivate === "true";
        policyCopy.IsOriginLinkEnable =
            policyCopy.IsOriginLinkEnable === "true";
        policyCopy.MaxSize = parseInt(policyCopy.MaxSize);
        policyCopy.OptionsSerialized.file_type = policyCopy.OptionsSerialized.file_type.split(
            ","
        );
        if (
            policyCopy.OptionsSerialized.file_type.length === 1 &&
            policyCopy.OptionsSerialized.file_type[0] === ""
        ) {
            policyCopy.OptionsSerialized.file_type = [];
        }

        API.post("/admin/policy", {
            policy: policyCopy,
        })
            .then(() => {
                ToggleSnackbar(
                    "top",
                    "right",
                    "儲存策略已" + (props.policy ? "儲存" : "添加"),
                    "success"
                );
            })
            .catch((error) => {
                ToggleSnackbar("top", "right", error.message, "error");
            })
            .then(() => {
                setLoading(false);
            });

        setLoading(false);
    };

    return (
        <div>
            <Typography variant={"h6"}>編輯儲存策略</Typography>
            <TableContainer>
                <form onSubmit={submitPolicy}>
                    <Table aria-label="simple table">
                        <TableHead>
                            <TableRow>
                                <TableCell>設定項</TableCell>
                                <TableCell>值</TableCell>
                                <TableCell>描述</TableCell>
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    ID
                                </TableCell>
                                <TableCell>{policy.ID}</TableCell>
                                <TableCell>儲存策略編號</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    類型
                                </TableCell>
                                <TableCell>{policy.Type}</TableCell>
                                <TableCell>儲存策略類型</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    名稱
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            required
                                            value={policy.Name}
                                            onChange={handleChange("Name")}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>儲存策名稱</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    Server
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            value={policy.Server}
                                            onChange={handleChange("Server")}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>儲存端 Endpoint</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    BucketName
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            value={policy.BucketName}
                                            onChange={handleChange(
                                                "BucketName"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>儲存桶標識</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    私有空間
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <RadioGroup
                                            required
                                            value={policy.IsPrivate}
                                            onChange={handleChange("IsPrivate")}
                                            row
                                        >
                                            <FormControlLabel
                                                value={"true"}
                                                control={
                                                    <Radio color={"primary"} />
                                                }
                                                label="是"
                                            />
                                            <FormControlLabel
                                                value={"false"}
                                                control={
                                                    <Radio color={"primary"} />
                                                }
                                                label="否"
                                            />
                                        </RadioGroup>
                                    </FormControl>
                                </TableCell>
                                <TableCell>是否為私有空間</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    文件資源根URL
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            value={policy.BaseURL}
                                            onChange={handleChange("BaseURL")}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>
                                    預覽/獲取文件外鏈時生成URL的前綴
                                </TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    AccessKey
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            rowsMax={10}
                                            value={policy.AccessKey}
                                            onChange={handleChange("AccessKey")}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>AccessKey / 重新整理Token</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    SecretKey
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            rowsMax={10}
                                            value={policy.SecretKey}
                                            onChange={handleChange("SecretKey")}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>SecretKey</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    最大單文件尺寸 (Bytes)
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            type={"number"}
                                            inputProps={{
                                                min: 0,
                                                step: 1,
                                            }}
                                            value={policy.MaxSize}
                                            onChange={handleChange("MaxSize")}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>
                                    最大可上傳的文件尺寸，填寫為0表示不限制
                                </TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    自動重新命名
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <RadioGroup
                                            required
                                            value={policy.AutoRename}
                                            onChange={handleChange(
                                                "AutoRename"
                                            )}
                                            row
                                        >
                                            <FormControlLabel
                                                value={"true"}
                                                control={
                                                    <Radio color={"primary"} />
                                                }
                                                label="是"
                                            />
                                            <FormControlLabel
                                                value={"false"}
                                                control={
                                                    <Radio color={"primary"} />
                                                }
                                                label="否"
                                            />
                                        </RadioGroup>
                                    </FormControl>
                                </TableCell>
                                <TableCell>
                                    是否根據規則對上傳物理文件重新命名
                                </TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    儲存路徑
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            value={policy.DirNameRule}
                                            onChange={handleChange(
                                                "DirNameRule"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>文件物理儲存路徑</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    儲存檔案名
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            value={policy.FileNameRule}
                                            onChange={handleChange(
                                                "FileNameRule"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>文件物理儲存檔案名</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    允許獲取外鏈
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <RadioGroup
                                            required
                                            value={policy.IsOriginLinkEnable}
                                            onChange={handleChange(
                                                "IsOriginLinkEnable"
                                            )}
                                            row
                                        >
                                            <FormControlLabel
                                                value={"true"}
                                                control={
                                                    <Radio color={"primary"} />
                                                }
                                                label="是"
                                            />
                                            <FormControlLabel
                                                value={"false"}
                                                control={
                                                    <Radio color={"primary"} />
                                                }
                                                label="否"
                                            />
                                        </RadioGroup>
                                    </FormControl>
                                </TableCell>
                                <TableCell>
                                    是否允許獲取外鏈。注意，某些儲存策略類型不支援，即使在此開啟，獲取的外鏈也無法使用。
                                </TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    又拍雲防盜鏈 Token
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            value={
                                                policy.OptionsSerialized.token
                                            }
                                            onChange={handleOptionChange(
                                                "token"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>僅對又拍雲端儲存策略有效</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    允許文件副檔名
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            value={
                                                policy.OptionsSerialized
                                                    .file_type
                                            }
                                            onChange={handleOptionChange(
                                                "file_type"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>留空表示不限制</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    允許的 MimeType
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            value={
                                                policy.OptionsSerialized
                                                    .mimetype
                                            }
                                            onChange={handleOptionChange(
                                                "mimetype"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>僅對七牛儲存策略有效</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    OneDrive 重定向地址
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            value={
                                                policy.OptionsSerialized
                                                    .od_redirect
                                            }
                                            onChange={handleOptionChange(
                                                "od_redirect"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>一般添加後無需修改</TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    OneDrive 反代伺服器地址
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            value={
                                                policy.OptionsSerialized
                                                    .od_proxy
                                            }
                                            onChange={handleOptionChange(
                                                "od_proxy"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>
                                    僅對 OneDrive 儲存策略有效
                                </TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    OneDrive/SharePoint 驅動器資源標識
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            value={
                                                policy.OptionsSerialized
                                                    .od_driver
                                            }
                                            onChange={handleOptionChange(
                                                "od_driver"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>
                                    僅對 OneDrive
                                    儲存策略有效，留空則使用使用者的預設 OneDrive
                                    驅動器
                                </TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    Amazon S3 Region
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            value={
                                                policy.OptionsSerialized.region
                                            }
                                            onChange={handleOptionChange(
                                                "region"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>
                                    僅對 Amazon S3 儲存策略有效
                                </TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell component="th" scope="row">
                                    內網 EndPoint
                                </TableCell>
                                <TableCell>
                                    <FormControl>
                                        <Input
                                            multiline
                                            value={
                                                policy.OptionsSerialized
                                                    .server_side_endpoint
                                            }
                                            onChange={handleOptionChange(
                                                "server_side_endpoint"
                                            )}
                                        />
                                    </FormControl>
                                </TableCell>
                                <TableCell>僅對 OSS 儲存策略有效</TableCell>
                            </TableRow>
                        </TableBody>
                    </Table>
                    <Button
                        type={"submit"}
                        color={"primary"}
                        variant={"contained"}
                        style={{ margin: 8 }}
                    >
                        儲存更改
                    </Button>
                </form>
            </TableContainer>
        </div>
    );
}
