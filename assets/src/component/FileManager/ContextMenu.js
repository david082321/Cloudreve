import {
    Divider,
    ListItemIcon,
    MenuItem,
    Typography,
    withStyles,
} from "@material-ui/core";
import Menu from "@material-ui/core/Menu";
import { Archive, InfoOutlined, Unarchive } from "@material-ui/icons";
import RenameIcon from "@material-ui/icons/BorderColor";
import DownloadIcon from "@material-ui/icons/CloudDownload";
import UploadIcon from "@material-ui/icons/CloudUpload";
import NewFolderIcon from "@material-ui/icons/CreateNewFolder";
import DeleteIcon from "@material-ui/icons/Delete";
import FileCopyIcon from "@material-ui/icons/FileCopy";
import OpenFolderIcon from "@material-ui/icons/FolderOpen";
import MoveIcon from "@material-ui/icons/Input";
import LinkIcon from "@material-ui/icons/InsertLink";
import OpenIcon from "@material-ui/icons/OpenInNew";
import ShareIcon from "@material-ui/icons/Share";
import { FolderUpload, MagnetOn, FilePlus } from "mdi-material-ui";
import PropTypes from "prop-types";
import React, { Component } from "react";
import { connect } from "react-redux";
import { withRouter } from "react-router-dom";
import {
    openCompressDialog,
    openCreateFileDialog,
    refreshFileList,
} from "../../actions";
import {
    changeContextMenu,
    navigateTo,
    openCopyDialog,
    openCreateFolderDialog,
    openDecompressDialog,
    openGetSourceDialog,
    openLoadingDialog,
    openMoveDialog,
    openMusicDialog,
    openRemoteDownloadDialog,
    openRemoveDialog,
    openRenameDialog,
    openShareDialog,
    openTorrentDownloadDialog,
    setNavigatorLoadingStatus,
    setSelectedTarget,
    showImgPreivew,
    toggleSnackbar,
} from "../../actions/index";
import { isCompressFile, isPreviewable, isTorrent } from "../../config";
import Auth from "../../middleware/Auth";
import { allowSharePreview } from "../../utils/index";
import pathHelper from "../../utils/page";
import RefreshIcon from "@material-ui/icons/Refresh";
import { openPreview } from "../../actions";
import {
    setSideBar,
    toggleObjectInfoSidebar,
} from "../../redux/explorer/action";

const styles = () => ({
    propover: {},
    divider: {
        marginTop: 4,
        marginBottom: 4,
    },
});

const StyledListItemIcon = withStyles({
    root: {
        minWidth: 38,
    },
})(ListItemIcon);

const mapStateToProps = (state) => {
    return {
        menuType: state.viewUpdate.contextType,
        menuOpen: state.viewUpdate.contextOpen,
        isMultiple: state.explorer.selectProps.isMultiple,
        withFolder: state.explorer.selectProps.withFolder,
        withFile: state.explorer.selectProps.withFile,
        path: state.navigator.path,
        selected: state.explorer.selected,
        keywords: state.explorer.keywords,
    };
};

const mapDispatchToProps = (dispatch) => {
    return {
        changeContextMenu: (type, open) => {
            dispatch(changeContextMenu(type, open));
        },
        setNavigatorLoadingStatus: (status) => {
            dispatch(setNavigatorLoadingStatus(status));
        },
        setSelectedTarget: (targets) => {
            dispatch(setSelectedTarget(targets));
        },
        navigateTo: (path) => {
            dispatch(navigateTo(path));
        },
        openCreateFolderDialog: () => {
            dispatch(openCreateFolderDialog());
        },
        openCreateFileDialog: () => {
            dispatch(openCreateFileDialog());
        },
        openRenameDialog: () => {
            dispatch(openRenameDialog());
        },
        openMoveDialog: () => {
            dispatch(openMoveDialog());
        },
        openRemoveDialog: () => {
            dispatch(openRemoveDialog());
        },
        openShareDialog: () => {
            dispatch(openShareDialog());
        },
        showImgPreivew: (first) => {
            dispatch(showImgPreivew(first));
        },
        openMusicDialog: () => {
            dispatch(openMusicDialog());
        },
        toggleSnackbar: (vertical, horizontal, msg, color) => {
            dispatch(toggleSnackbar(vertical, horizontal, msg, color));
        },
        openRemoteDownloadDialog: () => {
            dispatch(openRemoteDownloadDialog());
        },
        openTorrentDownloadDialog: () => {
            dispatch(openTorrentDownloadDialog());
        },
        openGetSourceDialog: () => {
            dispatch(openGetSourceDialog());
        },
        openCopyDialog: () => {
            dispatch(openCopyDialog());
        },
        openLoadingDialog: (text) => {
            dispatch(openLoadingDialog(text));
        },
        openDecompressDialog: () => {
            dispatch(openDecompressDialog());
        },
        openCompressDialog: () => {
            dispatch(openCompressDialog());
        },
        refreshFileList: () => {
            dispatch(refreshFileList());
        },
        openPreview: () => {
            dispatch(openPreview());
        },
        toggleObjectInfoSidebar: (open) => {
            dispatch(toggleObjectInfoSidebar(open));
        },
    };
};

class ContextMenuCompoment extends Component {
    X = 0;
    Y = 0;

    state = {};

    componentDidMount = () => {
        window.document.addEventListener("mousemove", this.setPoint);
    };

    setPoint = (e) => {
        this.Y = e.clientY;
        this.X = e.clientX;
    };

    openArchiveDownload = () => {
        this.props.changeContextMenu("file", false);
        this.props.openLoadingDialog("打包中...");
    };

    openDownload = () => {
        if (!allowSharePreview()) {
            this.props.toggleSnackbar(
                "top",
                "right",
                "未登入使用者無法預覽",
                "warning"
            );
            this.props.changeContextMenu("file", false);
            return;
        }
        this.props.changeContextMenu("file", false);
        this.props.openLoadingDialog("獲取下載網址...");
    };

    enterFolder = () => {
        this.props.navigateTo(
            this.props.path === "/"
                ? this.props.path + this.props.selected[0].name
                : this.props.path + "/" + this.props.selected[0].name
        );
    };

    clickUpload = (id) => {
        this.props.changeContextMenu("empty", false);
        const uploadButton = document.getElementsByClassName(id)[0];
        if (document.body.contains(uploadButton)) {
            uploadButton.click();
        } else {
            this.props.toggleSnackbar(
                "top",
                "right",
                "上傳元件還未載入完成",
                "warning"
            );
        }
    };

    // 暫時只對空白處右鍵選單使用這個函數，疑似有bug會導致的一個選單被預設選中。
    // 相關issue： https://github.com/mui-org/material-ui/issues/23747
    renderMenuItems = (items) => {
        const res = [];
        let key = 0;

        ["top", "center", "bottom"].forEach((position) => {
            let visibleCount = 0;
            items[position].forEach((item) => {
                if (item.condition) {
                    res.push(
                        <MenuItem dense key={key} onClick={item.onClick}>
                            <ListItemIcon>{item.icon}</ListItemIcon>
                            <Typography variant="inherit">
                                {item.text}
                            </Typography>
                        </MenuItem>
                    );
                    key++;
                    visibleCount++;
                }
            });
            if (visibleCount > 0 && position != "bottom") {
                res.push(
                    <Divider key={key} className={this.props.classes.divider} />
                );
                key++;
            }
        });

        return res;
    };

    render() {
        const { classes } = this.props;
        const user = Auth.GetUser();
        const isHomePage = pathHelper.isHomePage(this.props.location.pathname);
        const emptyMenuList = {
            top: [
                {
                    condition: true,
                    onClick: () => {
                        this.props.refreshFileList();
                        this.props.changeContextMenu(
                            this.props.menuType,
                            false
                        );
                    },
                    icon: <RefreshIcon />,
                    text: "重新整理",
                },
            ],
            center: [
                {
                    condition: true,
                    onClick: () => this.clickUpload("uploadFileForm"),
                    icon: <UploadIcon />,
                    text: "上傳文件",
                },
                {
                    condition: true,
                    onClick: () => this.clickUpload("uploadFolderForm"),
                    icon: <FolderUpload />,
                    text: "上傳目錄",
                },
                {
                    condition: user.group.allowRemoteDownload,
                    onClick: () => this.props.openRemoteDownloadDialog(),
                    icon: <DownloadIcon />,
                    text: "離線下載",
                },
            ],
            bottom: [
                {
                    condition: true,
                    onClick: () => this.props.openCreateFolderDialog(),
                    icon: <NewFolderIcon />,
                    text: "建立資料夾",
                },
                {
                    condition: true,
                    onClick: () => this.props.openCreateFileDialog(),
                    icon: <FilePlus />,
                    text: "建立文件",
                },
            ],
        };

        return (
            <div>
                <Menu
                    keepMounted
                    open={this.props.menuOpen}
                    onClose={() =>
                        this.props.changeContextMenu(this.props.menuType, false)
                    }
                    anchorReference="anchorPosition"
                    anchorPosition={{ top: this.Y, left: this.X }}
                    anchorOrigin={{
                        vertical: "top",
                        horizontal: "left",
                    }}
                    transformOrigin={{
                        vertical: "top",
                        horizontal: "left",
                    }}
                >
                    {this.props.menuType === "empty" && (
                        <div>
                            <MenuItem
                                dense
                                onClick={() => {
                                    this.props.refreshFileList();
                                    this.props.changeContextMenu(
                                        this.props.menuType,
                                        false
                                    );
                                }}
                            >
                                <StyledListItemIcon>
                                    <RefreshIcon />
                                </StyledListItemIcon>
                                <Typography variant="inherit">重新整理</Typography>
                            </MenuItem>
                            <Divider className={classes.divider} />
                            <MenuItem
                                dense
                                onClick={() =>
                                    this.clickUpload("uploadFileForm")
                                }
                            >
                                <StyledListItemIcon>
                                    <UploadIcon />
                                </StyledListItemIcon>
                                <Typography variant="inherit">
                                    上傳文件
                                </Typography>
                            </MenuItem>
                            <MenuItem
                                dense
                                onClick={() =>
                                    this.clickUpload("uploadFolderForm")
                                }
                            >
                                <StyledListItemIcon>
                                    <FolderUpload />
                                </StyledListItemIcon>
                                <Typography variant="inherit">
                                    上傳目錄
                                </Typography>
                            </MenuItem>
                            {user.group.allowRemoteDownload && (
                                <MenuItem
                                    dense
                                    onClick={() =>
                                        this.props.openRemoteDownloadDialog()
                                    }
                                >
                                    <StyledListItemIcon>
                                        <DownloadIcon />
                                    </StyledListItemIcon>
                                    <Typography variant="inherit">
                                        離線下載
                                    </Typography>
                                </MenuItem>
                            )}

                            <Divider className={classes.divider} />
                            <MenuItem
                                dense
                                onClick={() =>
                                    this.props.openCreateFolderDialog()
                                }
                            >
                                <StyledListItemIcon>
                                    <NewFolderIcon />
                                </StyledListItemIcon>
                                <Typography variant="inherit">
                                    建立資料夾
                                </Typography>
                            </MenuItem>
                            <MenuItem
                                dense
                                onClick={() =>
                                    this.props.openCreateFileDialog()
                                }
                            >
                                <StyledListItemIcon>
                                    <FilePlus />
                                </StyledListItemIcon>
                                <Typography variant="inherit">
                                    建立文件
                                </Typography>
                            </MenuItem>
                        </div>
                    )}
                    {this.props.menuType !== "empty" && (
                        <div>
                            {!this.props.isMultiple && this.props.withFolder && (
                                <div>
                                    <MenuItem dense onClick={this.enterFolder}>
                                        <StyledListItemIcon>
                                            <OpenFolderIcon />
                                        </StyledListItemIcon>
                                        <Typography variant="inherit">
                                            進入
                                        </Typography>
                                    </MenuItem>
                                    {isHomePage && (
                                        <Divider className={classes.divider} />
                                    )}
                                </div>
                            )}
                            {!this.props.isMultiple &&
                                this.props.withFile &&
                                (!this.props.share ||
                                    this.props.share.preview) &&
                                isPreviewable(this.props.selected[0].name) && (
                                    <div>
                                        <MenuItem
                                            dense
                                            onClick={() =>
                                                this.props.openPreview()
                                            }
                                        >
                                            <StyledListItemIcon>
                                                <OpenIcon />
                                            </StyledListItemIcon>
                                            <Typography variant="inherit">
                                                打開
                                            </Typography>
                                        </MenuItem>
                                    </div>
                                )}

                            {!this.props.isMultiple && this.props.withFile && (
                                <div>
                                    <MenuItem
                                        dense
                                        onClick={() => this.openDownload()}
                                    >
                                        <StyledListItemIcon>
                                            <DownloadIcon />
                                        </StyledListItemIcon>
                                        <Typography variant="inherit">
                                            下載
                                        </Typography>
                                    </MenuItem>
                                    {isHomePage && (
                                        <Divider className={classes.divider} />
                                    )}
                                </div>
                            )}

                            {(this.props.isMultiple || this.props.withFolder) &&
                                (user.group.allowArchiveDownload ||
                                    !isHomePage) && (
                                    <MenuItem
                                        dense
                                        onClick={() =>
                                            this.openArchiveDownload()
                                        }
                                    >
                                        <StyledListItemIcon>
                                            <DownloadIcon />
                                        </StyledListItemIcon>
                                        <Typography variant="inherit">
                                            打包下載
                                        </Typography>
                                    </MenuItem>
                                )}

                            {!this.props.isMultiple &&
                                this.props.withFile &&
                                isHomePage &&
                                user.policy.allowSource && (
                                    <MenuItem
                                        dense
                                        onClick={() =>
                                            this.props.openGetSourceDialog()
                                        }
                                    >
                                        <StyledListItemIcon>
                                            <LinkIcon />
                                        </StyledListItemIcon>
                                        <Typography variant="inherit">
                                            獲取外鏈
                                        </Typography>
                                    </MenuItem>
                                )}

                            {!this.props.isMultiple &&
                                isHomePage &&
                                user.group.allowRemoteDownload &&
                                this.props.withFile &&
                                isTorrent(this.props.selected[0].name) && (
                                    <MenuItem
                                        dense
                                        onClick={() =>
                                            this.props.openTorrentDownloadDialog()
                                        }
                                    >
                                        <StyledListItemIcon>
                                            <MagnetOn />
                                        </StyledListItemIcon>
                                        <Typography variant="inherit">
                                            建立離線下載任務
                                        </Typography>
                                    </MenuItem>
                                )}
                            {!this.props.isMultiple &&
                                isHomePage &&
                                user.group.compress &&
                                this.props.withFile &&
                                isCompressFile(this.props.selected[0].name) && (
                                    <MenuItem
                                        dense
                                        onClick={() =>
                                            this.props.openDecompressDialog()
                                        }
                                    >
                                        <StyledListItemIcon>
                                            <Unarchive />
                                        </StyledListItemIcon>
                                        <Typography variant="inherit">
                                            解壓縮
                                        </Typography>
                                    </MenuItem>
                                )}

                            {isHomePage && user.group.compress && (
                                <MenuItem
                                    dense
                                    onClick={() =>
                                        this.props.openCompressDialog()
                                    }
                                >
                                    <StyledListItemIcon>
                                        <Archive />
                                    </StyledListItemIcon>
                                    <Typography variant="inherit">
                                        建立壓縮文件
                                    </Typography>
                                </MenuItem>
                            )}

                            {!this.props.isMultiple && isHomePage && (
                                <MenuItem
                                    dense
                                    onClick={() => this.props.openShareDialog()}
                                >
                                    <StyledListItemIcon>
                                        <ShareIcon />
                                    </StyledListItemIcon>
                                    <Typography variant="inherit">
                                        建立分享連結
                                    </Typography>
                                </MenuItem>
                            )}

                            {!this.props.isMultiple && isHomePage && (
                                <MenuItem
                                    dense
                                    onClick={() =>
                                        this.props.toggleObjectInfoSidebar(true)
                                    }
                                >
                                    <StyledListItemIcon>
                                        <InfoOutlined />
                                    </StyledListItemIcon>
                                    <Typography variant="inherit">
                                        詳細訊息
                                    </Typography>
                                </MenuItem>
                            )}

                            {!this.props.isMultiple && isHomePage && (
                                <Divider className={classes.divider} />
                            )}

                            {!this.props.isMultiple && isHomePage && (
                                <div>
                                    <MenuItem
                                        dense
                                        onClick={() =>
                                            this.props.openRenameDialog()
                                        }
                                    >
                                        <StyledListItemIcon>
                                            <RenameIcon />
                                        </StyledListItemIcon>
                                        <Typography variant="inherit">
                                            重新命名
                                        </Typography>
                                    </MenuItem>
                                    {this.props.keywords === "" && (
                                        <MenuItem
                                            dense
                                            onClick={() =>
                                                this.props.openCopyDialog()
                                            }
                                        >
                                            <StyledListItemIcon>
                                                <FileCopyIcon />
                                            </StyledListItemIcon>
                                            <Typography variant="inherit">
                                                複製
                                            </Typography>
                                        </MenuItem>
                                    )}
                                </div>
                            )}
                            {isHomePage && (
                                <div>
                                    {this.props.keywords === "" && (
                                        <MenuItem
                                            dense
                                            onClick={() =>
                                                this.props.openMoveDialog()
                                            }
                                        >
                                            <StyledListItemIcon>
                                                <MoveIcon />
                                            </StyledListItemIcon>
                                            <Typography variant="inherit">
                                                移動
                                            </Typography>
                                        </MenuItem>
                                    )}

                                    <Divider className={classes.divider} />
                                    <MenuItem
                                        dense
                                        className={classes.propover}
                                        onClick={() =>
                                            this.props.openRemoveDialog()
                                        }
                                    >
                                        <StyledListItemIcon>
                                            <DeleteIcon />
                                        </StyledListItemIcon>
                                        <Typography variant="inherit">
                                            刪除
                                        </Typography>
                                    </MenuItem>
                                </div>
                            )}
                        </div>
                    )}
                </Menu>
            </div>
        );
    }
}

ContextMenuCompoment.propTypes = {
    classes: PropTypes.object.isRequired,
    menuType: PropTypes.string.isRequired,
};

const ContextMenu = connect(
    mapStateToProps,
    mapDispatchToProps
)(withStyles(styles)(withRouter(ContextMenuCompoment)));

export default ContextMenu;
